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

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/ksonnet/kubecfg/pkg/kubecfg"
)

const (
	flagCreate       = "create"
	flagSkipGc       = "skip-gc"
	flagGcTag        = "gc-tag"
	flagGcListMode   = "gc-list-mode"
	flagGcNsSelector = "gc-ns-selector"
	flagDryRun       = "dry-run"
)

func init() {
	RootCmd.AddCommand(updateCmd)
	updateCmd.PersistentFlags().Bool(flagCreate, true, "Create missing resources")
	updateCmd.PersistentFlags().Bool(flagSkipGc, false, "Don't perform garbage collection, even with --"+flagGcTag)
	updateCmd.PersistentFlags().String(flagGcTag, "", "Add this tag to updated objects, and garbage collect existing objects with this tag and not in config")
	updateCmd.PersistentFlags().String(flagGcListMode, kubecfg.GcListModeClusterScope, fmt.Sprintf("Selects the way the GC list all objects: %s,%s", kubecfg.GcListModeClusterScope, kubecfg.GcListModePerNamespace))
	updateCmd.PersistentFlags().String(flagGcNsSelector, "", fmt.Sprintf("Use this label selector to filter namespaces. Implies %s=%s ", flagGcListMode, kubecfg.GcListModePerNamespace))
	updateCmd.PersistentFlags().Bool(flagDryRun, false, "Perform only read-only operations")
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update Kubernetes resources with local config",
	RunE: func(cmd *cobra.Command, args []string) error {
		flags := cmd.Flags()
		var err error
		c := kubecfg.UpdateCmd{}

		c.Create, err = flags.GetBool(flagCreate)
		if err != nil {
			return err
		}

		c.GcTag, err = flags.GetString(flagGcTag)
		if err != nil {
			return err
		}

		c.SkipGc, err = flags.GetBool(flagSkipGc)
		if err != nil {
			return err
		}

		c.GcListMode, err = flags.GetString(flagGcListMode)
		if err != nil {
			return err
		}

		c.GcNsSelector, err = flags.GetString(flagGcNsSelector)
		if err != nil {
			return err
		}
		if c.GcNsSelector != "" {
			c.GcListMode = kubecfg.GcListModePerNamespace
		}

		c.DryRun, err = flags.GetBool(flagDryRun)
		if err != nil {
			return err
		}

		c.ClientPool, c.Discovery, err = restClientPool(cmd)
		if err != nil {
			return err
		}

		c.DefaultNamespace, err = defaultNamespace(clientConfig)
		if err != nil {
			return err
		}

		cc, err := clientConfig.ClientConfig()
		if err != nil {
			return err
		}

		c.CoreV1Client, err = v1.NewForConfig(cc)
		if err != nil {
			return err
		}

		objs, err := readObjs(cmd, args)
		if err != nil {
			return err
		}

		return c.Run(objs)
	},
}
