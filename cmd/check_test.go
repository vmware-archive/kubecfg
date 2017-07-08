package cmd

import (
	"testing"
	"encoding/json"
	"k8s.io/client-go/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/validation"
)

func TestCorrectObject(t *testing.T) {
	var obj runtime.Unstructured
	data := []byte(`{
  "apiVersion": "v1",
  "kind": "Service",
  "metadata": {
    "name": "kafka",
    "namespace": "kubeless"
  },
  "spec": {
    "ports": [
      {
        "port": 9092
      }
    ],
    "selector": {
      "app": "kafka"
    }
  }
}`)

	if err := json.Unmarshal(data, &obj.Object); err != nil {
		t.Errorf("Test failed due to: %v", err)
	}

	groupVersion := obj.GetAPIVersion()
	schemaData, err := downloadSchema(groupVersion)
	if err != nil {
		t.Errorf("Test failed due to: %v", err)
	}

	// Load schema
	schema, err := validation.NewSwaggerSchemaFromBytes(schemaData, nil)
	if err != nil {
		t.Errorf("Test failed due to: %v", err)
	}

	// Validate obj
	objData, err := json.Marshal(obj.Object)
	if err != nil {
		t.Errorf("Test failed due to: %v", err)
	}
	err = schema.ValidateBytes(objData)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
}

func TestIncorrectObject(t *testing.T) {
	var obj runtime.Unstructured
	data := []byte(`{
  "apiVersion": "v1",
  "kind": "TestObject",
  "nil": null,
  "bool": true,
  "number": 42,
  "string": "bar",
  "notAVal": "aVal",
  "notAnotherVal": "aVal2",
  "filevar": "foo\n",
  "array": ["one", 2, [3]],
  "object": {"foo": "bar"}
}`)

	if err := json.Unmarshal(data, &obj.Object); err != nil {
		t.Errorf("Test failed due to: %v", err)
	}

	groupVersion := obj.GetAPIVersion()
	schemaData, err := downloadSchema(groupVersion)
	if err != nil {
		t.Errorf("Test failed due to: %v", err)
	}

	// Load schema
	schema, err := validation.NewSwaggerSchemaFromBytes(schemaData, nil)
	if err != nil {
		t.Errorf("Test failed due to: %v", err)
	}

	// Validate obj
	objData, err := json.Marshal(obj.Object)
	if err != nil {
		t.Errorf("Test failed due to: %v", err)
	}
	err = schema.ValidateBytes(objData)
	if err == nil {
		// err should be "couldn't find type: v1.TestObject"
		t.Errorf("Expected %v, got nil", err)
	}
}