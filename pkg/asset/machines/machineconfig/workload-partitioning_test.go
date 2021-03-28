package machineconfig

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	igntypes "github.com/coreos/ignition/v2/config/v3_2/types"
	"github.com/stretchr/testify/assert"
	"github.com/vincent-petithory/dataurl"

	"github.com/openshift/installer/pkg/types"
)

func expectedCrioCfg(partitions []types.WorkloadPartition) string {
	parts := []string{}
	for _, p := range partitions {
		parts = append(parts, fmt.Sprintf(`[crio.runtime.workloads.%[1]s]
label             = "%[1]s.workload.openshift.io/cores"
annotation_prefix = "io.openshift.workload.%[1]s"
resources         = { "cpu" = "", "cpuset" = "%s", }
`, p.Name, p.CPUIds))
	}
	return strings.Join(parts, "\n") + "\n"
}

func expectedKubeletCfg(partitions []types.WorkloadPartition) string {
	parts := []string{}
	for _, p := range partitions {
		parts = append(parts, fmt.Sprintf(`  %q: {
    "cpuset": %q
  }`, p.Name, p.CPUIds))
	}
	return "{\n" + strings.Join(parts, ",\n") + "\n}"
}

func TestWorkloadPartitioning(t *testing.T) {
	cases := []struct {
		partitions []types.WorkloadPartition
		role       string
	}{
		{
			partitions: []types.WorkloadPartition{
				{
					Name:   types.ManagementWorkloadPartition,
					CPUIds: "0-1",
				},
			},
			role: "master",
		},
		{
			partitions: []types.WorkloadPartition{
				{
					Name:   types.ManagementWorkloadPartition,
					CPUIds: "0-1",
				},
				{
					Name:   "secondary",
					CPUIds: "50-51",
				},
			},
			role: "master",
		},
	}

	t.Run("test", func(t *testing.T) {
		for _, tc := range cases {
			expectedCrioCfg := expectedCrioCfg(tc.partitions)
			crioCfg, err := CrioWorkloadDropinContents(tc.partitions)
			assert.Equal(t, err, nil, "No err")
			assert.Equal(t, expectedCrioCfg, crioCfg)

			expectedKubeletCfg := expectedKubeletCfg(tc.partitions)
			kubeletCfg, err := KubeletWorkloadDropinContents(tc.partitions)
			assert.Equal(t, err, nil, "No err")
			assert.Equal(t, expectedKubeletCfg, kubeletCfg)

			result, err := ForWorkloadPartitions(tc.partitions, tc.role)
			assert.Equal(t, err, nil, "No err")
			assert.Equal(t, tc.role, result.ObjectMeta.Labels["machineconfiguration.openshift.io/role"])

			cfg := igntypes.Config{}
			err = json.Unmarshal(result.Spec.Config.Raw, &cfg)
			assert.Equal(t, err, nil, "No err")
			files := cfg.Storage.Files
			assert.Equal(t, 2, len(files), "Two files in the machineconfig")
			assert.Equal(t, "/etc/crio/crio.conf.d/01-workload-partitioning", files[0].Path, "Cri-O drop-in is present")
			assert.Equal(t, dataurl.EncodeBytes([]byte(expectedCrioCfg)), *files[0].Contents.Source)
			assert.Equal(t, "/etc/kubernetes/workload-pinning", files[1].Path, "Kubelet drop-in is present")
			assert.Equal(t, dataurl.EncodeBytes([]byte(expectedKubeletCfg)), *files[1].Contents.Source)
		}
	})
}
