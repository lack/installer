package machineconfig

import (
	"encoding/json"
	"fmt"
	"testing"

	igntypes "github.com/coreos/ignition/v2/config/v3_2/types"

	"github.com/stretchr/testify/assert"
	"github.com/vincent-petithory/dataurl"
)

func TestWorkloadPartitioning(t *testing.T) {
	name := "master"
	cpuset := "0-1"
	role := "role"

	expectedCrioCfg := fmt.Sprintf(`[crio.runtime.workloads.%[1]s]
label             = "workload.openshift.io/%[1]s/cpu"
annotation_prefix = "io.openshift.workload.%[1]s"
resources         = { "cpu" = "", "cpuset" = "%s", }

`, name, cpuset)

	expectedKubeletCfg := fmt.Sprintf(`{
  %q: {
    "cpuset": %q
  }
}`, name, cpuset)

	t.Run("test", func(t *testing.T) {
		crioCfg, err := crioConfig(name, cpuset)
		assert.Equal(t, err, nil, "No err")
		assert.Equal(t, expectedCrioCfg, crioCfg)

		kubeletCfg, err := kubeletConfig(name, cpuset)
		assert.Equal(t, err, nil, "No err")
		assert.Equal(t, expectedKubeletCfg, kubeletCfg)

		result, err := ForWorkloadPartitioning(name, cpuset, role)
		assert.Equal(t, err, nil, "No err")
		cfg := igntypes.Config{}
		err = json.Unmarshal(result.Spec.Config.Raw, &cfg)
		assert.Equal(t, err, nil, "No err")
		files := cfg.Storage.Files
		assert.Equal(t, 2, len(files), "Two files in the machineconfig")
		assert.Equal(t, fmt.Sprintf("/etc/crio/crio.conf.d/01-%s-workload", name), files[0].Path, "Cri-o config is present")
		assert.Equal(t, dataurl.EncodeBytes([]byte(expectedCrioCfg)), *files[0].Contents.Source)
		assert.Equal(t, "/etc/kubernetes/workload-pinning", files[1].Path, "Kubeleto config is present")
	})
}
