package machineconfig

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/BurntSushi/toml"
	igntypes "github.com/coreos/ignition/v2/config/v3_2/types"
	mcfgv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/installer/pkg/asset/ignition"
	"github.com/openshift/installer/pkg/types"
)

// CrioWorkloadDropinContents generates the content expected by Cri-O for workload partitioning
//
// Example output:
// [crio.runtime.workloads.management]
// label = "management.workload.openshift.io/cores"
// annotation_prefix = "io.openshift.workload.management"
// resources = { "cpu" = "", "cpuset" = "0-1", }
func CrioWorkloadDropinContents(partitions []types.WorkloadPartition) (string, error) {
	crioWorkloadDropin := map[string]crioWorkloadCfg{}
	for _, p := range partitions {
		crioWorkloadDropin[fmt.Sprintf("crio.runtime.workloads.%s", p.Name)] = crioWorkloadCfg{
			Label:            fmt.Sprintf("%s.workload.openshift.io/cores", p.Name),
			AnnotationPrefix: fmt.Sprintf("io.openshift.workload.%s", p.Name),
			Resources:        fmt.Sprintf(`{ "cpu" = "", "cpuset" = "%s", }`, p.CPUIds),
		}
	}
	buf := new(bytes.Buffer)
	if err := toml.NewEncoder(buf).Encode(crioWorkloadDropin); err != nil {
		return "", errors.Wrap(err, "Could not encode TOML")
	}
	return buf.String(), nil
}

type crioWorkloadCfg struct {
	Label            string `toml:"label"`
	AnnotationPrefix string `toml:"annotation_prefix"`
	Resources        string `toml:"resources"`
}

// KubeletWorkloadDropinContents generates te content expected by Kubelet for workload partitioning
//
// Example output:
// {
//   "management": {
//     "cpuset": "0-1"
//   }
// }
func KubeletWorkloadDropinContents(partitions []types.WorkloadPartition) (string, error) {
	kubeletWorkload := map[string]kubeletWorkloadEntry{}
	for _, p := range partitions {
		kubeletWorkload[string(p.Name)] = kubeletWorkloadEntry{Cpuset: p.CPUIds}
	}
	kubeletCfg, err := json.MarshalIndent(kubeletWorkload, "", "  ")
	if err != nil {
		return "", errors.Wrap(err, "Could not marshall JSON")
	}
	return string(kubeletCfg), nil
}

type kubeletWorkloadEntry struct {
	Cpuset string `json:"cpuset"`
}

// ForWorkloadPartitions creates the MachineConfig that configures Cri-O and
// Kubelet with the workload partition from install-config.yaml
// 'workloadSettings'
func ForWorkloadPartitions(partitions []types.WorkloadPartition, role string) (*mcfgv1.MachineConfig, error) {
	crioCfg, err := CrioWorkloadDropinContents(partitions)
	if err != nil {
		return nil, err
	}
	kubeletCfg, err := KubeletWorkloadDropinContents(partitions)
	if err != nil {
		return nil, err
	}
	ignConfig := igntypes.Config{
		Ignition: igntypes.Ignition{
			Version: igntypes.MaxVersion.String(),
		},
		Storage: igntypes.Storage{
			Files: []igntypes.File{
				ignition.FileFromString(
					"/etc/crio/crio.conf.d/01-workload-partitioning",
					"root", 0644, crioCfg),
				ignition.FileFromString(
					"/etc/kubernetes/workload-pinning",
					"root", 0644, kubeletCfg),
			},
		},
	}
	rawExt, err := ignition.ConvertToRawExtension(ignConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Could not convert to raw extension")
	}

	return &mcfgv1.MachineConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: mcfgv1.SchemeGroupVersion.String(),
			Kind:       "MachineConfig",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("02-%s-workload-partitioning", role),
			Labels: map[string]string{
				"machineconfiguration.openshift.io/role": role,
			},
		},
		Spec: mcfgv1.MachineConfigSpec{
			Config: rawExt,
		},
	}, nil
}
