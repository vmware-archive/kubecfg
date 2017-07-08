package cmd

import (
	"encoding/json"
	"fmt"
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

		for _, obj := range objs {
			groupVersion := obj.GetAPIVersion()

			// Download schemaData for obj
			schemaData, err := downloadSchema(groupVersion)
			if err != nil {
				return err
			}

			// Load schema
			schema, err := validation.NewSwaggerSchemaFromBytes(schemaData, nil)
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

		fmt.Println("The manifest is valid")
		return nil
	},
}

func downloadSchema(groupVersion string) ([]byte, error) {
	prefix := "apis"
	if groupVersion == "v1" {
		prefix = "api"
	}

	_, disco, err := restClientPool(nil)
	if err != nil {
		return []byte{}, err
	}

	schemaData, err := disco.RESTClient().Get().
		AbsPath("/swaggerapi", prefix, groupVersion).
		Do().
		Raw()

	return schemaData, err
}
