package machineconfig

import (
	"bytes"
	"encoding/json"
	"fmt"

	igntypes "github.com/coreos/ignition/v2/config/v3_2/types"
	mcfgv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"
	ini "gopkg.in/ini.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/installer/pkg/asset/ignition"
	"github.com/pkg/errors"
)

type KubeletWorkloadEntry struct {
	Cpuset string `json:"cpuset"`
}
type KubeletWorkload map[string]KubeletWorkloadEntry

type CrioCfg struct {
	Label            string `ini:"label"`
	AnnotationPrefix string `ini:"annotation_prefix"`
	Resources        string `ini:"resources"`
}

func crioConfig(name, cpuset string) (string, error) {
	/*
		[crio.runtime.workloads.management]
		label = "workload.openshift.io/management/cpu"
		annotation_prefix = "io.openshift.workload.management"
		resources = { "cpu" = "", "cpuset" = "0-1", }
	*/
	crioIni := ini.Empty()
	section := crioIni.Section(fmt.Sprintf("crio.runtime.workloads.%s", name))
	err := section.ReflectFrom(&CrioCfg{
		Label:            fmt.Sprintf(`"workload.openshift.io/%s/cpu"`, name),
		AnnotationPrefix: fmt.Sprintf(`"io.openshift.workload.%1s"`, name),
		Resources:        fmt.Sprintf(`{ "cpu" = "", "cpuset" = "%s", }`, cpuset),
	})
	if err != nil {
		return "", errors.Wrap(err, "Could not reflect structure to INI")
	}
	crioBuf := new(bytes.Buffer)
	_, err = crioIni.WriteTo(crioBuf)
	if err != nil {
		return "", errors.Wrap(err, "Could not write INI to buffer")
	}
	return crioBuf.String(), nil
}

func kubeletConfig(name, cpuset string) (string, error) {
	kubeletCfg, err := json.MarshalIndent(KubeletWorkload{
		name: KubeletWorkloadEntry{
			Cpuset: cpuset,
		},
	}, "", "  ")
	if err != nil {
		return "", errors.Wrap(err, "Could not marshall JSON")
	}
	return string(kubeletCfg), nil
}

func ForWorkloadPartitioning(name, cpuset, role string) (*mcfgv1.MachineConfig, error) {
	crioCfg, err := crioConfig(name, cpuset)
	if err != nil {
		return nil, err
	}
	kubeletCfg, err := kubeletConfig(name, cpuset)
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
					fmt.Sprintf("/etc/crio/crio.conf.d/01-%s-workload", name),
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
