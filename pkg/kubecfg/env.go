package kubecfg

import (
	"github.com/ksonnet/kubecfg/metadata"
)

type EnvAddCmd struct {
	name string
	uri  string

	rootPath metadata.AbsPath
	spec     metadata.ClusterSpec
}

func NewEnvAddCmd(name, uri, specFlag string, rootPath metadata.AbsPath) (*EnvAddCmd, error) {
	spec, err := metadata.ParseClusterSpec(specFlag)
	if err != nil {
		return nil, err
	}

	return &EnvAddCmd{name: name, uri: uri, spec: spec, rootPath: rootPath}, nil
}

func (c *EnvAddCmd) Run() error {
	manager, err := metadata.Find(c.rootPath)
	if err != nil {
		return err
	}

	extensionsLibData, k8sLibData, err := manager.GenerateKsonnetLibData(c.spec)
	if err != nil {
		return err
	}

	return manager.CreateEnvironment(c.name, c.uri, c.spec, extensionsLibData, k8sLibData)
}
