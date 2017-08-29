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

package kubecfg

import (
	"fmt"
	"io"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/discovery"

	"github.com/ksonnet/kubecfg/template"
	"github.com/ksonnet/kubecfg/utils"
)

// ValidateCmd represents the validate subcommand
type ValidateCmd struct {
	Discovery discovery.DiscoveryInterface

	Expander    *template.Expander
	Environment *string
	Files       []string
}

func (c ValidateCmd) Run(out io.Writer) error {
	hasError := false

	objs, err := c.Expander.Expand(c.Files)
	if err != nil {
		return err
	}

	for _, obj := range objs {
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
