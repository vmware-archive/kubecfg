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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"

	"github.com/ksonnet/kubecfg/utils"
)

// ValidateCmd represents the validate subcommand
type ValidateCmd struct {
	Discovery discovery.DiscoveryInterface
}

func (c ValidateCmd) Run(apiObjects []*unstructured.Unstructured, out io.Writer) error {
	knownResources := map[string]sets.String{}

	serverResourceList, err := c.Discovery.ServerResources()
	if err != nil {
		return err
	}
	for _, rl := range serverResourceList {
		knownResources[rl.GroupVersion] = sets.String{}
		for _, r := range rl.APIResources {
			knownResources[rl.GroupVersion][r.Name] = sets.Empty{}
		}
	}

	hasError := false

	for _, obj := range apiObjects {
		resName := utils.ResourceNameFor(c.Discovery, obj)
		desc := fmt.Sprintf("%s %s", resName, utils.FqName(obj))
		log.Info("Validating ", desc)

		gv := obj.GroupVersionKind().GroupVersion()

		var allErrs []error

		schema, err := utils.NewSwaggerSchemaFor(c.Discovery, gv)
		if err != nil {
			if _, known := knownResources[gv.String()][resName]; errors.IsNotFound(err) && known {
				log.Warnf("Skipping validation of known resource %s %s that lacks a registered schema", gv, resName)
				continue
			}
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
