package utils

import (
	"testing"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/version"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input    version.Info
		expected ServerVersion
		error    bool
	}{
		{
			input:    version.Info{Major: "1", Minor: "6"},
			expected: ServerVersion{Major: 1, Minor: 6},
		},
		{
			input:    version.Info{Major: "1", Minor: "70"},
			expected: ServerVersion{Major: 1, Minor: 70},
		},
		{
			input: version.Info{Major: "1", Minor: "6x"},
			error: true,
		},
		{
			input:    version.Info{Major: "1", Minor: "8+"},
			expected: ServerVersion{Major: 1, Minor: 8},
		},
		{
			input:    version.Info{Major: "", Minor: "", GitVersion: "v1.8.0"},
			expected: ServerVersion{Major: 1, Minor: 8},
		},
		{
			input:    version.Info{Major: "1", Minor: "", GitVersion: "v1.8.0"},
			expected: ServerVersion{Major: 1, Minor: 8},
		},
		{
			input:    version.Info{Major: "", Minor: "8", GitVersion: "v1.8.0"},
			expected: ServerVersion{Major: 1, Minor: 8},
		},
		{
			input:    version.Info{Major: "", Minor: "", GitVersion: "v1.8.8-test.0"},
			expected: ServerVersion{Major: 1, Minor: 8},
		},
		{
			input:    version.Info{Major: "1", Minor: "8", GitVersion: "v1.9.0"},
			expected: ServerVersion{Major: 1, Minor: 8},
		},
		{
			input: version.Info{Major: "", Minor: "", GitVersion: "v1.a"},
			error: true,
		},
	}

	for _, test := range tests {
		v, err := ParseVersion(&test.input)
		if test.error {
			if err == nil {
				t.Errorf("test %s should have failed and did not", test.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("test %v failed: %v", test.input, err)
			continue
		}
		if v != test.expected {
			t.Errorf("Expected %v, got %v", test.expected, v)
		}
	}
}

func TestVersionCompare(t *testing.T) {
	v := ServerVersion{Major: 2, Minor: 3}
	tests := []struct {
		major, minor, result int
	}{
		{major: 1, minor: 0, result: 1},
		{major: 2, minor: 0, result: 1},
		{major: 2, minor: 2, result: 1},
		{major: 2, minor: 3, result: 0},
		{major: 2, minor: 4, result: -1},
		{major: 3, minor: 0, result: -1},
	}
	for _, test := range tests {
		res := v.Compare(test.major, test.minor)
		if res != test.result {
			t.Errorf("%d.%d => Expected %d, got %d", test.major, test.minor, test.result, res)
		}
	}
}

func TestResourceNameFor(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tests/v1alpha1",
			"kind":       "Test",
			"metadata": map[string]interface{}{
				"name":      "myname",
				"namespace": "mynamespace",
			},
		},
	}

	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
	mapper.Add(schema.GroupVersionKind{Group: "tests", Version: "v1alpha1", Kind: "Test"}, meta.RESTScopeNamespace)

	if n := ResourceNameFor(mapper, obj); n != "tests" {
		t.Errorf("Got resource name %q for %v", n, obj)
	}

	obj.SetKind("Unknown")
	if n := ResourceNameFor(mapper, obj); n != "unknown" {
		t.Errorf("Got resource name %q for %v", n, obj)
	}

	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "unknown", Version: "noversion", Kind: "SomeKind"})
	if n := ResourceNameFor(mapper, obj); n != "somekind" {
		t.Errorf("Got resource name %q for %v", n, obj)
	}
}

func TestFqName(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tests/v1alpha1",
			"kind":       "Test",
			"metadata": map[string]interface{}{
				"name": "myname",
			},
		},
	}

	if n := FqName(obj); n != "myname" {
		t.Errorf("Got %q for %v", n, obj)
	}

	obj.SetNamespace("mynamespace")
	if n := FqName(obj); n != "mynamespace.myname" {
		t.Errorf("Got %q for %v", n, obj)
	}
}

func TestCompactEncodeRoundTrip(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tests/v1alpha1",
			"kind":       "Test",
			"metadata": map[string]interface{}{
				"name": "myname",
			},
			"foo": true,
		},
	}

	data, err := CompactEncodeObject(obj)
	if err != nil {
		t.Errorf("CompactEncodeObject returned %v", err)
	}
	t.Logf("compact encoding is %d bytes", len(data))

	out := &unstructured.Unstructured{}
	if err := CompactDecodeObject(data, out); err != nil {
		t.Errorf("CompactDecodeObject returned %v", err)
	}

	t.Logf("in:  %#v", obj)
	t.Logf("out: %#v", out)
	if !apiequality.Semantic.DeepEqual(obj, out) {
		t.Error("Objects differed: ", diff.ObjectDiff(obj, out))
	}
}
