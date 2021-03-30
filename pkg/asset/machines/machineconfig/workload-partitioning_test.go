package machineconfig

import (
	"encoding/json"
	"fmt"
	"testing"

	igntypes "github.com/coreos/ignition/v2/config/v3_2/types"
	"github.com/stretchr/testify/assert"
	"github.com/vincent-petithory/dataurl"

	"github.com/openshift/installer/pkg/types"
)

func TestWorkloadPartitioning(t *testing.T) {
	partitions := []types.WorkloadPartition{
		{
			Name:   types.ManagementWorkloadPartition,
			CPUIds: "0-1",
		},
	}
	name := partitions[0].Name
	cpuset := partitions[0].CPUIds
	role := "role"

	expectedCrioCfg := fmt.Sprintf(`["crio.runtime.workloads.%[1]s"]
  label = "%[1]s.workload.openshift.io/cores"
  annotation_prefix = "io.openshift.workload.%[1]s"
  resources = "{ \"cpu\" = \"\", \"cpuset\" = \"%s\", }"
`, name, cpuset)

	expectedKubeletCfg := fmt.Sprintf(`{
  %q: {
    "cpuset": %q
  }
}`, name, cpuset)

	t.Run("test", func(t *testing.T) {
		crioCfg, err := CrioWorkloadDropinContents(partitions)
		assert.Equal(t, err, nil, "No err")
		assert.Equal(t, expectedCrioCfg, crioCfg)

		kubeletCfg, err := KubeletWorkloadDropinContents(partitions)
		assert.Equal(t, err, nil, "No err")
		assert.Equal(t, expectedKubeletCfg, kubeletCfg)

		result, err := ForWorkloadPartitions(partitions, role)
		assert.Equal(t, err, nil, "No err")
		cfg := igntypes.Config{}
		err = json.Unmarshal(result.Spec.Config.Raw, &cfg)
		assert.Equal(t, err, nil, "No err")
		files := cfg.Storage.Files
		assert.Equal(t, 2, len(files), "Two files in the machineconfig")
		assert.Equal(t, "/etc/crio/crio.conf.d/01-workload-partitioning", files[0].Path, "Cri-O drop-in is present")
		assert.Equal(t, dataurl.EncodeBytes([]byte(expectedCrioCfg)), *files[0].Contents.Source)
		assert.Equal(t, "/etc/kubernetes/workload-pinning", files[1].Path, "Kubelet drop-in is present")
		assert.Equal(t, dataurl.EncodeBytes([]byte(expectedKubeletCfg)), *files[1].Contents.Source)
	})
}
