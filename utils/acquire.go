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
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	jsonnet "github.com/google/go-jsonnet"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// Read fetches and decodes K8s objects by path.
// TODO: Replace this with something supporting more sophisticated
// content negotiation.
func Read(vm *jsonnet.VM, path string) ([]runtime.Object, error) {
	ext := filepath.Ext(path)
	// Double any single quotes in the path so they are quoted in the produced expression
	quotedPath := strings.Replace(path, "'", "''", -1)
	var expr string
	switch ext {
	case ".json":
		expr = fmt.Sprintf(`(import "internal:///kubecfg.libsonnet").parseJson(importstr @'%s')`, quotedPath)
	case ".yaml":
		expr = fmt.Sprintf(`(import "internal:///kubecfg.libsonnet").parseYaml(importstr @'%s')`, quotedPath)
	case ".jsonnet":
		expr = fmt.Sprintf("import @'%s'", quotedPath)
	default:
		// Keep unquoted path in return error to not confuse anyone
		return nil, fmt.Errorf("Unknown file extension: %s", path)
	}
	return jsonnetReader(vm, expr)
}

type walkContext struct {
	parent *walkContext
	label  string
}

func (c *walkContext) String() string {
	parent := ""
	if c.parent != nil {
		parent = c.parent.String()
	}
	return parent + c.label
}

func jsonWalk(parentCtx *walkContext, obj interface{}) ([]interface{}, error) {
	switch o := obj.(type) {
	case nil:
		return []interface{}{}, nil
	case map[string]interface{}:
		if o["kind"] != nil && o["apiVersion"] != nil {
			return []interface{}{o}, nil
		}
		ret := []interface{}{}
		for k, v := range o {
			ctx := walkContext{
				parent: parentCtx,
				label:  "." + k,
			}
			children, err := jsonWalk(&ctx, v)
			if err != nil {
				return nil, err
			}
			ret = append(ret, children...)
		}
		return ret, nil
	case []interface{}:
		ret := make([]interface{}, 0, len(o))
		for i, v := range o {
			ctx := walkContext{
				parent: parentCtx,
				label:  fmt.Sprintf("[%d]", i),
			}
			children, err := jsonWalk(&ctx, v)
			if err != nil {
				return nil, err
			}
			ret = append(ret, children...)
		}
		return ret, nil
	default:
		return nil, fmt.Errorf("Looking for kubernetes object at %s, but instead found %T", parentCtx, o)
	}
}

func jsonnetReader(vm *jsonnet.VM, expr string) ([]runtime.Object, error) {
	jsonstr, err := vm.EvaluateSnippet("", expr)
	if err != nil {
		return nil, err
	}

	log.Debugf("jsonnet result is: %s", jsonstr)

	var top interface{}
	if err = json.Unmarshal([]byte(jsonstr), &top); err != nil {
		return nil, err
	}

	objs, err := jsonWalk(&walkContext{label: "<top>"}, top)
	if err != nil {
		return nil, err
	}

	ret := make([]runtime.Object, 0, len(objs))
	for _, v := range objs {
		obj := &unstructured.Unstructured{Object: v.(map[string]interface{})}
		if obj.IsList() {
			// TODO: Use obj.ToList with newer apimachinery
			list := &unstructured.UnstructuredList{
				Object: obj.Object,
			}
			err := obj.EachListItem(func(item runtime.Object) error {
				castItem := item.(*unstructured.Unstructured)
				list.Items = append(list.Items, *castItem)
				return nil
			})
			if err != nil {
				return nil, err
			}
			ret = append(ret, list)
		} else {
			ret = append(ret, obj)
		}
	}

	return ret, nil
}

// FlattenToV1 expands any List-type objects into their members, and
// cooerces everything to v1.Unstructured.  Panics if coercion
// encounters an unexpected object type.
func FlattenToV1(objs []runtime.Object) []*unstructured.Unstructured {
	ret := make([]*unstructured.Unstructured, 0, len(objs))
	for _, obj := range objs {
		switch o := obj.(type) {
		case *unstructured.UnstructuredList:
			for i := range o.Items {
				ret = append(ret, &o.Items[i])
			}
		case *unstructured.Unstructured:
			ret = append(ret, o)
		default:
			panic("Unexpected unstructured object type")
		}
	}
	return ret
}
