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

package utils

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"
	openapi_v2 "github.com/googleapis/gnostic/openapiv2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type schemaFromFile struct {
	dir string
}

func (s schemaFromFile) OpenAPISchema() (*openapi_v2.Document, error) {
	var doc openapi_v2.Document
	b, err := ioutil.ReadFile(filepath.Join(s.dir, "schema.pb"))
	if err != nil {
		return nil, err
	}
	if err := proto.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func TestValidate(t *testing.T) {
	schemaReader := schemaFromFile{dir: filepath.FromSlash("../testdata")}
	s, err := NewOpenAPISchemaFor(schemaReader, schema.GroupVersionKind{Version: "v1", Kind: "Service"})
	if err != nil {
		t.Fatalf("Error reading schema: %v", err)
	}

	valid := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"spec": map[string]interface{}{
				"ports": []interface{}{
					map[string]interface{}{"port": 80},
				},
			},
		},
	}
	if errs := s.Validate(valid); len(errs) != 0 {
		t.Errorf("schema errors from valid object: %v", errs)
	}

	invalid := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"spec": map[string]interface{}{
				"bogus": false,
				"ports": []interface{}{
					map[string]interface{}{"port": "bogus"},
				},
			},
		},
	}
	errs := s.Validate(invalid)
	if len(errs) == 0 {
		t.Error("no schema errors from invalid object :(")
	}
	err = utilerrors.NewAggregate(errs)
	t.Logf("Invalid object produced error: %v", err)

	if !strings.Contains(err.Error(), `invalid type for io.k8s.api.core.v1.ServicePort.port: got "string", expected "integer"`) {
		t.Errorf("Wrong error1 produced from invalid object: %v", err)
	}
	if !strings.Contains(err.Error(), `ValidationError(v1.Service.spec): unknown field "bogus" in io.k8s.api.core.v1.ServiceSpec`) {
		t.Errorf("Wrong error2 produced from invalid object: %q", err)
	}
}
