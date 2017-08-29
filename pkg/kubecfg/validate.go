package kubecfg

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"

	"github.com/ksonnet/kubecfg/utils"
)

type ValidateCmd struct {
	Discovery discovery.DiscoveryInterface

	Objs []*unstructured.Unstructured
}

func (c ValidateCmd) Run() error {
	hasError := false

	for _, obj := range c.Objs {
		desc := fmt.Sprintf("%s %s", utils.ResourceNameFor(c.Discovery, obj), utils.FqName(obj))
		log.Info("Validating ", desc)

		var allErrs []error

		schema, err := utils.NewSwaggerSchemaFor(c.Discovery, obj.GroupVersionKind().GroupVersion())
		if err != nil {
			allErrs = append(allErrs, fmt.Errorf("Unable to fetch schema: %v", err))
		} else {
			// Validate obj
			allErrs = append(allErrs, schema.Validate(obj)...)
		}

		for _, err := range allErrs {
			log.Errorf("Error in %s: %v", desc, err)
			hasError = true
		}
	}

	if hasError {
		return fmt.Errorf("Validation failed")
	}

	return nil
}
