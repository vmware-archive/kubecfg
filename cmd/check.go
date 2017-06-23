package cmd

import (
	"encoding/json"
	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/api/validation"
)

func init() {
	RootCmd.AddCommand(checkCmd)
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Compare generated manifest against Swagger spec",
	RunE: func(cmd *cobra.Command, args []string) error {
		objs, err := readObjs(cmd, args)
		if err != nil {
			return err
		}
		_, disco, err := restClientPool(cmd)
		if err != nil {
			return err
		}
		for _, obj := range objs {
			groupVersion := obj.GetAPIVersion()
			prefix := "apis"
			if groupVersion == "v1" {
				prefix = "api"
			}

			// Download schemaData for obj
			schemaData, err := disco.RESTClient().Get().
				AbsPath("/swaggerapi", prefix, groupVersion).
				Do().
				Raw()
			if err != nil {
				return err
			}

			// Load schema
			schema, err := validation.NewSwaggerSchemaFromBytes(schemaData, validation.Schema{})
			if err != nil {
				return err
			}

			// Validate obj
			objData, err := json.Marshal(obj)
			if err != nil {
				return err
			}
			err = schema.ValidateBytes(objData)
			if err != nil {
				return err
			}
		}

		return nil
	},
}
