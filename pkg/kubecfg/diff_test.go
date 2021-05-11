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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestLastAppliedStrategy(t *testing.T) {

	var liveMap map[string]interface{}
	var expectedMap map[string]interface{}

	c := DiffCmd{}
	c.DiffStrategy = "last-applied"

	liveText :=
		`{
           "apiVersion": "v1",
           "kind": "Service",
           "metadata": {
             "name": "foo",
             "namespace": "default",
             "resourceVersion": "288527730",
             "selfLink": "/api/v1/namespaces/default/services/foo",
             "uid": "687c86fe-12ec-45d1-a4e8-61db473e2d01",
             "creationTimestamp": "2021-04-11T13:38:06Z",
             "annotations":  {
               "testKey": "testValue",
               "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"annotations\":{\"testKey\": \"testValue\"},\"name\":\"foo\",\"namespace\":\"default\"}, \"spec\":{\"ports\":[{\"name\":\"http\",\"port\":8080}],\"selector\":{\"app\":\"foo\"}}}"
             }
           },
           "spec": {
             "sessionAffinity": "None",
             "type": "ClusterIP",
             "selector": {
               "app": "foo"
             },
             "ports": [
               {
                 "name": "http",
                 "port": 8080,
                 "protocol": "TCP",
                 "targetPort": 8080
               }
             ]
           },
           "status": {
             "loadBalancer": {}
           }
         }`
	json.Unmarshal([]byte(liveText), &liveMap)

	expectedText :=
		`{
           "apiVersion": "v1",
           "kind": "Service",
           "metadata": {
             "name": "foo",
             "namespace": "default",
             "annotations":  {
               "testKey": "testValue"
             }
           },
           "spec": {
             "selector": {
               "app": "foo"
             },
             "ports": [
               {
                 "name": "http",
                 "port": 8080
               }
             ]
           }
         }`
	json.Unmarshal([]byte(expectedText), &expectedMap)
	liveObj, err := c.getLiveObjObject(nil, &unstructured.Unstructured{Object: liveMap})
	if err != nil {
		t.Error(err)
	}
	require.Equal(t, expectedMap, liveObj)
}

func TestLastAppliedStrategyKubecfgAnnotation(t *testing.T) {

	var liveMap map[string]interface{}
	var expectedMap map[string]interface{}

	c := DiffCmd{}
	c.DiffStrategy = "last-applied"

	liveText :=
		`{
           "apiVersion": "v1",
           "kind": "Service",
           "metadata": {
             "name": "foo",
             "namespace": "default",
             "resourceVersion": "288527730",
             "selfLink": "/api/v1/namespaces/default/services/foo",
             "uid": "687c86fe-12ec-45d1-a4e8-61db473e2d01",
             "creationTimestamp": "2021-04-11T13:38:06Z",
             "annotations":  {
               "testKey": "testValue",
               "kubecfg.ksonnet.io/last-applied-configuration": "H4sIANVcmWAAAzWOsQrDMBBD935F0Jwh3YJ/oWMhS+lwOBdqmtjGvgRC8L/3nLabJKSHDlB0A6fsgofBdkWLt/Oj6junzVnWYGGhkYRgDpD3QUi0nqsVznLjHaY55UDzyigtPC2sjCkEfE2OZGsy8kTrLNppkCPbCokhidIex3/2Eom6qzlM3/VdebbIPLOVkM4XMf7opZTLB1jX/izFAAAA"
             }
           },
           "spec": {
             "sessionAffinity": "None",
             "type": "ClusterIP",
             "selector": {
               "app": "foo"
             },
             "ports": [
               {
                 "name": "http",
                 "port": 8080,
                 "protocol": "TCP",
                 "targetPort": 8080
               }
             ]
           },
           "status": {
             "loadBalancer": {}
           }
         }`
	if err := json.Unmarshal([]byte(liveText), &liveMap); err != nil {
		t.Error(err)
	}

	expectedText :=
		`{
           "apiVersion": "v1",
           "kind": "Service",
           "metadata": {
             "name": "foo",
             "namespace": "default",
             "annotations":  {
               "testKey": "testValue"
             }
           },
           "spec": {
             "selector": {
               "app": "foo"
             },
             "ports": [
               {
                 "name": "http",
                 "port": 8080
               }
             ]
           }
         }`
	if err := json.Unmarshal([]byte(expectedText), &expectedMap); err != nil {
		t.Error(err)
	}
	liveObj, err := c.getLiveObjObject(nil, &unstructured.Unstructured{Object: liveMap})
	if err != nil {
		t.Error(err)
	}
	require.Equal(t, expectedMap, liveObj)
}

func TestRemoveListFields(t *testing.T) {
	for _, tc := range []struct {
		config, live, expected []interface{}
	}{
		{
			config:   []interface{}{"a"},
			live:     []interface{}{"a"},
			expected: []interface{}{"a"},
		},

		// Check that extra fields in config are not propagated.
		{
			config:   []interface{}{"a", "b"},
			live:     []interface{}{"a"},
			expected: []interface{}{"a"},
		},

		// Check that extra entries in live are propagated.
		{
			config:   []interface{}{"a"},
			live:     []interface{}{"a", "b"},
			expected: []interface{}{"a", "b"},
		},
	} {
		require.EqualValues(t, tc.expected, removeListFields(tc.config, tc.live))
	}
}

func TestRemoveMapFields(t *testing.T) {
	for _, tc := range []struct {
		config, live, expected map[string]interface{}
	}{
		{
			config:   map[string]interface{}{"foo": "bar"},
			live:     map[string]interface{}{"foo": "bar"},
			expected: map[string]interface{}{"foo": "bar"},
		},

		{
			config:   map[string]interface{}{"foo": "bar", "bar": "baz"},
			live:     map[string]interface{}{"foo": "bar"},
			expected: map[string]interface{}{"foo": "bar"},
		},

		{
			config:   map[string]interface{}{"foo": "bar"},
			live:     map[string]interface{}{"foo": "bar", "bar": "baz"},
			expected: map[string]interface{}{"foo": "bar"},
		},
	} {
		require.Equal(t, tc.expected, removeMapFields(tc.config, tc.live))
	}
}

func TestRemoveFields(t *testing.T) {
	emptyVal := map[string]interface{}{
		"args":    map[string]interface{}{},
		"volumes": []string{},
		"stdin":   false,
	}
	for _, tc := range []struct {
		config, live, expected interface{}
	}{
		// Check we can handle embedded structs.
		{
			config:   map[string]interface{}{"foo": "bar", "bar": "baz"},
			live:     map[string]interface{}{"foo": "bar"},
			expected: map[string]interface{}{"foo": "bar"},
		},
		// JSON unmarshalling can return int64 for numbers
		// https://golang.org/pkg/encoding/json/#Number
		{
			config:   map[string]interface{}{"foo": (int64)(10)},
			live:     map[string]interface{}{},
			expected: map[string]interface{}{},
		},

		// Check we can handle embedded lists.
		{
			config:   []interface{}{"a", "b"},
			live:     []interface{}{"a"},
			expected: []interface{}{"a"},
		},

		// Check we can handle arbitrary types.
		{
			config:   "a",
			live:     "b",
			expected: "b",
		},
		// Check we can handle mismatched types.
		{
			config:   map[string]interface{}{"foo": "bar"},
			live:     []interface{}{"foo", "bar"},
			expected: []interface{}{"foo", "bar"},
		},
		{
			config:   []interface{}{"foo", "bar"},
			live:     map[string]interface{}{"foo": "bar"},
			expected: map[string]interface{}{"foo": "bar"},
		},
		// Check we handle empty configs by copying them as if were live
		// (API won't return them)
		{
			config:   emptyVal,
			live:     map[string]interface{}{},
			expected: emptyVal,
		},

		// Check we can handle combinations.
		{
			config: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Service",
				"metadata": map[string]interface{}{
					"name":      "foo",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"selector": map[string]interface{}{
						"name": "foo",
					},
					"ports": []interface{}{
						map[string]interface{}{
							"name": "http",
							"port": 80,
						},
						map[string]interface{}{
							"name": "https",
							"port": 443,
						},
					},
				},
			},
			live: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Service",
				"metadata": map[string]interface{}{
					"name": "foo",
					// NB Namespace missing.
				},
				"spec": map[string]interface{}{
					"selector": map[string]interface{}{
						"bar": "foo",
					},
					"ports": []interface{}{
						// NB HTTP port missing.
						map[string]interface{}{
							"name": "https",
							"port": 443,
						},
					},
				},
			},
			expected: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Service",
				"metadata": map[string]interface{}{
					"name": "foo",
				},
				"spec": map[string]interface{}{
					"selector": map[string]interface{}{},
					"ports": []interface{}{
						map[string]interface{}{
							"name": "https",
							"port": 443,
						},
					},
				},
			},
		},
	} {
		require.Equal(t, tc.expected, removeFields(tc.config, tc.live))
	}
}
