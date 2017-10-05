// Copyright 2017 The kubecfg authors
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/ksonnet/kubecfg/metadata"
	"github.com/ksonnet/kubecfg/pkg/kubecfg"
)

const flagDiffStrategy = "diff-strategy"

func init() {
	addEnvCmdFlags(diffCmd)
	bindClientGoFlags(diffCmd)
	bindJsonnetFlags(diffCmd)
	diffCmd.PersistentFlags().String(flagDiffStrategy, "all", "Diff strategy, all or subset.")
	RootCmd.AddCommand(diffCmd)
}

var diffCmd = &cobra.Command{
	Use:   "diff [env-name] [env-name] [-f <file-or-dir>]",
	Short: "Display differences between server and local config, or server and server config",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 2 {
			return fmt.Errorf("'diff' takes at most two argument, that is the name of the environments")
		}

		flags := cmd.Flags()

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		wd := metadata.AbsPath(cwd)

		envSpec, err := parseEnvCmd(cmd, args)
		if err != nil {
			return err
		}

		diffStrategy, err := flags.GetString(flagDiffStrategy)
		if err != nil {
			return err
		}

		c, err := initDiffCmd(cmd, wd, envSpec, diffStrategy)
		if err != nil {
			return err
		}

		return c.Run(cmd.OutOrStdout())
	},
	Long: `Display differences between server and local configuration, or server and server
configurations.

ksonnet applications are accepted, as well as normal JSON, YAML, and Jsonnet
files.`,
	Example: `  # Show diff between resources described in a the local 'dev' environment
  # specified by the ksonnet application and the remote cluster referenced by
  # the same 'dev' environment. Can be used in any subdirectory of the application.
  ksonnet diff dev

  # Show diff between resources at remote clusters. This requires ksonnet
  # application defined environments. Diff between the cluster defined at the
  # 'us-west/dev' environment, and the cluster defined at the 'us-west/prod'
  # environment. Can be used in any subdirectory of the application.
  ksonnet diff remote:us-west/dev remote:us-west/prod

  # Show diff between resources at a remote and a local cluster. This requires
  # ksonnet application defined environments. Diff between the cluster defined
  # at the 'us-west/dev' environment, and the cluster defined at the
  # 'us-west/prod' environment. Can be used in any subdirectory of the
  # application.
  ksonnet diff local:us-west/dev remote:us-west/prod

  # Show diff between resources described in a YAML file and the cluster
  # referenced in '$KUBECONFIG'.
  ksonnet diff -f ./pod.yaml

  # Show diff between resources described in a JSON file and the cluster
  # referenced by the environment 'dev'.
  ksonnet diff dev -f ./pod.json

  # Show diff between resources described in a YAML file and the cluster
  # referred to by './kubeconfig'.
  ksonnet diff --kubeconfig=./kubeconfig -f ./pod.yaml`,
}

func initDiffCmd(cmd *cobra.Command, wd metadata.AbsPath, envSpec *envSpec, diffStrategy string) (kubecfg.DiffCmd, error) {
	const (
		remote = "remote"
		local  = "local"
	)

	var err error

	// ---------------------------------------------------------------------------
	// Diff between expanded Kubernete objects and objects on a remote cluster
	// ---------------------------------------------------------------------------
	if envSpec.env2 == nil {
		c := kubecfg.DiffRemoteCmd{}
		c.DiffStrategy = diffStrategy
		c.Client = &kubecfg.Client{}

		c.Client.APIObjects, err = expandEnvCmdObjs(cmd, envSpec, wd)
		if err != nil {
			return nil, err
		}

		c.Client.ClientPool, c.Client.Discovery, err = restClientPool(cmd, envSpec.env)
		if err != nil {
			return nil, err
		}

		c.Client.Namespace, err = defaultNamespace(clientConfig)
		if err != nil {
			return nil, err
		}

		return &c, nil
	}

	env1 := strings.SplitN(*envSpec.env, ":", 2)
	env2 := strings.SplitN(*envSpec.env2, ":", 2)

	if len(env1) < 2 || len(env2) < 2 || (env1[0] != local && env1[0] != remote) || (env2[0] != local && env2[0] != remote) {
		return nil, fmt.Errorf("[env-name] must be prefaced by %s: or %s:, ex: %s:us-west/prod", local, remote, remote)
	}

	// ---------------------------------------------------------------------------
	// Diff between two sets of expanded Kubernete objects
	// ---------------------------------------------------------------------------
	if env1[0] == local && env2[0] == local {
		c := kubecfg.DiffLocalCmd{}
		c.DiffStrategy = diffStrategy

		manager, err := metadata.Find(wd)
		if err != nil {
			return nil, err
		}

		baseObj, err := constructBaseObj(manager)
		if err != nil {
			return nil, err
		}

		c.Env1 = &kubecfg.LocalEnv{}
		c.Env1.Name = env1[1]
		c.Env1.APIObjects, err = expandEnvObjs(cmd, c.Env1.Name, baseObj, manager)
		if err != nil {
			return nil, err
		}

		c.Env2 = &kubecfg.LocalEnv{}
		c.Env2.Name = env2[1]
		c.Env2.APIObjects, err = expandEnvObjs(cmd, c.Env2.Name, baseObj, manager)
		if err != nil {
			return nil, err
		}

		return &c, nil
	}

	return nil, nil
}

// expandEnvObjs finds and expands templates for an environment
func expandEnvObjs(cmd *cobra.Command, env, baseObj string, manager metadata.Manager) ([]*unstructured.Unstructured, error) {
	expander, err := newExpander(cmd)
	if err != nil {
		return nil, err
	}

	libPath, envLibPath, envComponentPath := manager.LibPaths(env)
	expander.FlagJpath = append([]string{string(libPath), string(envLibPath)}, expander.FlagJpath...)
	expander.ExtCodes = append([]string{baseObj}, expander.ExtCodes...)

	envFiles := []string{string(envComponentPath)}

	return expander.Expand(envFiles)
}
