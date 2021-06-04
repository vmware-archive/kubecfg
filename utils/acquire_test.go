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
	"reflect"
	"sort"
	"testing"
)

func TestJsonWalk(t *testing.T) {
	fooObj := map[string]interface{}{
		"apiVersion": "test",
		"kind":       "Foo",
	}
	barObj := map[string]interface{}{
		"apiVersion": "test",
		"kind":       "Bar",
	}

	tests := []struct {
		input  string
		result []interface{}
		error  string
	}{
		{
			// nil input
			input:  `null`,
			result: []interface{}{},
		},
		{
			// single basic object
			input:  `{"apiVersion": "test", "kind": "Foo"}`,
			result: []interface{}{fooObj},
		},
		{
			// array of objects
			input:  `[{"apiVersion": "test", "kind": "Foo"}, {"apiVersion": "test", "kind": "Bar"}]`,
			result: []interface{}{barObj, fooObj},
		},
		{
			// object of objects
			input:  `{"foo": {"apiVersion": "test", "kind": "Foo"}, "bar": {"apiVersion": "test", "kind": "Bar"}}`,
			result: []interface{}{barObj, fooObj},
		},
		{
			// Deeply nested
			input:  `{"foo": [[{"apiVersion": "test", "kind": "Foo"}], {"apiVersion": "test", "kind": "Bar"}]}`,
			result: []interface{}{barObj, fooObj},
		},
		{
			// Error: nested misplaced value
			input: `{"foo": {"bar": [null, 42]}}`,
			error: "Looking for kubernetes object at <top>.foo.bar[1], but instead found float64",
		},
	}

	for i, test := range tests {
		t.Logf("%d: %s", i, test.input)
		var top interface{}
		if err := json.Unmarshal([]byte(test.input), &top); err != nil {
			t.Errorf("Failed to unmarshal %q: %v", test.input, err)
			continue
		}
		objs, err := jsonWalk(&walkContext{label: "<top>"}, top)
		if test.error != "" {
			// expect error
			if err == nil {
				t.Errorf("Test %d failed to fail", i)
			} else if err.Error() != test.error {
				t.Errorf("Test %d failed with %q but expected %q", i, err, test.error)
			}

			continue
		}

		// expect success
		if err != nil {
			t.Errorf("Test %d failed: %v", i, err)
			continue
		}
		keyFunc := func(i int) string {
			v := objs[i].(map[string]interface{})
			return v["kind"].(string)
		}
		sort.Slice(objs, func(i, j int) bool {
			return keyFunc(i) < keyFunc(j)
		})
		if !reflect.DeepEqual(objs, test.result) {
			t.Errorf("Expected %v, got %v", test.result, objs)
		}
	}
}
