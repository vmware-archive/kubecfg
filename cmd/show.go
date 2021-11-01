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
	"github.com/spf13/cobra"

	"github.com/bitnami/kubecfg/pkg/kubecfg"
)

const (
	flagFormat               = "format"
	flagExportDir            = "export-dir"
	flagExportFileNameFormat = "export-filename-format"
)

func init() {
	RootCmd.AddCommand(showCmd)
	showCmd.PersistentFlags().StringP(flagFormat, "o", "yaml", "Output format.  Supported values are: json, yaml")
	showCmd.PersistentFlags().String(flagExportDir, "", "Split yaml stream into multiple files and write files into a directory. If the directory exists it must be empty.")
	showCmd.PersistentFlags().String(flagExportFileNameFormat, kubecfg.DefaultFileNameFormat, "Go template expression used to render path names for resources.")

}

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show expanded resource definitions",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		flags := cmd.Flags()

		outputFormat, err := flags.GetString(flagFormat)
		if err != nil {
			return err
		}
		exportDir, err := flags.GetString(flagExportDir)
		if err != nil {
			return err
		}
		exportFileNameFormat, err := flags.GetString(flagExportFileNameFormat)
		if err != nil {
			return err
		}

		c, err := kubecfg.NewShowCmd(outputFormat, exportDir, exportFileNameFormat)
		if err != nil {
			return err
		}

		objs, err := readObjs(cmd, args)
		if err != nil {
			return err
		}

		return c.Run(objs, cmd.OutOrStdout())
	},
}
