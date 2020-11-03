package kubecfg

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	pb_proto "github.com/golang/protobuf/proto"
	openapi_v2 "github.com/googleapis/gnostic/openapiv2"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/kube-openapi/pkg/util/proto"
	"k8s.io/kubectl/pkg/util/openapi"

	"github.com/bitnami/kubecfg/utils"
)

func TestStringListContains(t *testing.T) {
	t.Parallel()
	foobar := []string{"foo", "bar"}
	if stringListContains([]string{}, "") {
		t.Error("Empty list was not empty")
	}
	if !stringListContains(foobar, "foo") {
		t.Error("Failed to find foo")
	}
	if stringListContains(foobar, "baz") {
		t.Error("Should not contain baz")
	}
}

func TestIsValidKindSchema(t *testing.T) {
	t.Parallel()
	schemaResources := readSchemaOrDie(filepath.FromSlash("../../testdata/schema.pb"))

	cmgvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}
	if !isValidKindSchema(schemaResources.LookupResource(cmgvk)) {
		t.Errorf("%s should have a valid schema", cmgvk)
	}

	if isValidKindSchema(nil) {
		t.Error("nil should not be a valid schema")
	}

	// This is what a schema-less CRD appears as in k8s >= 1.15
	mapSchema := &proto.Map{
		BaseSchema: proto.BaseSchema{
			Extensions: map[string]interface{}{
				"x-kubernetes-group-version-kind": []interface{}{
					map[interface{}]interface{}{"group": "bitnami.com", "kind": "SealedSecret", "version": "v1alpha1"},
				},
			},
		},
		SubType: &proto.Arbitrary{},
	}
	if isValidKindSchema(mapSchema) {
		t.Error("Trivial type:object schema should be invalid")
	}
}

func TestEligibleForGc(t *testing.T) {
	t.Parallel()
	const myTag = "my-gctag"
	boolTrue := true
	o := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tests/v1alpha1",
			"kind":       "Dummy",
		},
	}

	if eligibleForGc(o, myTag) {
		t.Errorf("%v should not be eligible (no tag)", o)
	}

	// [gctag-migration]: Remove annotation in phase2
	utils.SetMetaDataAnnotation(o, AnnotationGcTag, "unknowntag")
	utils.SetMetaDataLabel(o, LabelGcTag, "unknowntag")
	if eligibleForGc(o, myTag) {
		t.Errorf("%v should not be eligible (wrong tag)", o)
	}

	// [gctag-migration]: Remove annotation in phase2
	utils.SetMetaDataAnnotation(o, AnnotationGcTag, myTag)
	utils.SetMetaDataLabel(o, LabelGcTag, myTag)
	if !eligibleForGc(o, myTag) {
		t.Errorf("%v should be eligible", o)
	}

	// [gctag-migration]: Remove testcase in phase2
	utils.SetMetaDataAnnotation(o, AnnotationGcTag, myTag)
	utils.DeleteMetaDataLabel(o, LabelGcTag) // no label. ie: pre-migration
	if !eligibleForGc(o, myTag) {
		t.Errorf("%v should be eligible (gctag-migration phase1)", o)
	}

	utils.SetMetaDataAnnotation(o, AnnotationGcStrategy, GcStrategyIgnore)
	if eligibleForGc(o, myTag) {
		t.Errorf("%v should not be eligible (strategy=ignore)", o)
	}

	utils.SetMetaDataAnnotation(o, AnnotationGcStrategy, GcStrategyAuto)
	if !eligibleForGc(o, myTag) {
		t.Errorf("%v should be eligible (strategy=auto)", o)
	}

	// Unstructured.SetOwnerReferences is broken in apimachinery release-1.6
	// See kubernetes/kubernetes#46817
	setOwnerRef := func(u *unstructured.Unstructured, ref metav1.OwnerReference) {
		// This is not a complete nor robust reimplementation
		c := map[string]interface{}{
			"kind": ref.Kind,
			"name": ref.Name,
		}
		if ref.Controller != nil {
			c["controller"] = *ref.Controller
		}
		u.Object["metadata"].(map[string]interface{})["ownerReferences"] = []interface{}{c}
	}
	setOwnerRef(o, metav1.OwnerReference{Kind: "foo", Name: "bar"})
	if !eligibleForGc(o, myTag) {
		t.Errorf("%v should be eligible (non-controller ownerref)", o)
	}

	setOwnerRef(o, metav1.OwnerReference{Kind: "foo", Name: "bar", Controller: &boolTrue})
	if eligibleForGc(o, myTag) {
		t.Errorf("%v should not be eligible (controller ownerref)", o)
	}
}

func exampleConfigMap() *unstructured.Unstructured {
	result := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "myname",
				"namespace": "mynamespace",
				"annotations": map[string]interface{}{
					"myannotation": "somevalue",
				},
			},
			"data": map[string]interface{}{
				"foo": "bar",
			},
		},
	}

	return result
}

func addOrigAnnotation(obj *unstructured.Unstructured) {
	data, err := utils.CompactEncodeObject(obj)
	if err != nil {
		panic(fmt.Sprintf("Failed to serialise object: %v", err))
	}
	utils.SetMetaDataAnnotation(obj, AnnotationOrigObject, data)
}

func newPatchMetaFromStructOrDie(dataStruct interface{}) strategicpatch.PatchMetaFromStruct {
	t, err := strategicpatch.NewPatchMetaFromStruct(dataStruct)
	if err != nil {
		panic(fmt.Sprintf("NewPatchMetaFromStruct(%t) failed: %v", dataStruct, err))
	}
	return t
}

func readSchemaOrDie(path string) openapi.Resources {
	var doc openapi_v2.Document
	b, err := ioutil.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("Unable to read %s: %v", path, err))
	}
	if err := pb_proto.Unmarshal(b, &doc); err != nil {
		panic(fmt.Sprintf("Unable to unmarshal %s: %v", path, err))
	}
	schemaResources, err := openapi.NewOpenAPIData(&doc)
	if err != nil {
		panic(fmt.Sprintf("Unable to parse openapi doc: %v", err))
	}
	return schemaResources
}

func TestPatchNoop(t *testing.T) {
	t.Parallel()
	schemaResources := readSchemaOrDie(filepath.FromSlash("../../testdata/schema.pb"))

	existing := exampleConfigMap()
	new := existing.DeepCopy()
	addOrigAnnotation(existing)

	result, err := patch(existing, new, schemaResources.LookupResource(existing.GroupVersionKind()))
	if err != nil {
		t.Errorf("patch() returned error: %v", err)
	}

	t.Logf("existing: %#v", existing)
	t.Logf("result: %#v", result)
	if !apiequality.Semantic.DeepEqual(existing, result) {
		t.Error("Objects differed: ", diff.ObjectDiff(existing, result))
	}
}

func TestPatchNoopNoAnnotation(t *testing.T) {
	t.Parallel()
	schemaResources := readSchemaOrDie(filepath.FromSlash("../../testdata/schema.pb"))

	existing := exampleConfigMap()
	new := existing.DeepCopy()
	// Note: no addOrigAnnotation(existing)

	result, err := patch(existing, new, schemaResources.LookupResource(existing.GroupVersionKind()))
	if err != nil {
		t.Errorf("patch() returned error: %v", err)
	}

	// result should == existing, except for annotation

	if result.GetAnnotations()[AnnotationOrigObject] == "" {
		t.Errorf("result lacks last-applied annotation")
	}

	utils.DeleteMetaDataAnnotation(result, AnnotationOrigObject)
	if !apiequality.Semantic.DeepEqual(existing, result) {
		t.Error("Objects differed: ", diff.ObjectDiff(existing, result))
	}
}

func TestPatchNoConflict(t *testing.T) {
	t.Parallel()
	schemaResources := readSchemaOrDie(filepath.FromSlash("../../testdata/schema.pb"))

	existing := exampleConfigMap()
	utils.SetMetaDataAnnotation(existing, "someanno", "origvalue")
	addOrigAnnotation(existing)
	utils.SetMetaDataAnnotation(existing, "otheranno", "existingvalue")
	new := exampleConfigMap()
	utils.SetMetaDataAnnotation(new, "someanno", "newvalue")

	result, err := patch(existing, new, schemaResources.LookupResource(existing.GroupVersionKind()))
	if err != nil {
		t.Errorf("patch() returned error: %v", err)
	}

	t.Logf("existing: %#v", existing)
	t.Logf("result: %#v", result)
	someanno := result.GetAnnotations()["someanno"]
	if someanno != "newvalue" {
		t.Errorf("someanno was %q", someanno)
	}

	otheranno := result.GetAnnotations()["otheranno"]
	if otheranno != "existingvalue" {
		t.Errorf("otheranno was %q", otheranno)
	}
}

func TestPatchConflict(t *testing.T) {
	t.Parallel()
	schemaResources := readSchemaOrDie(filepath.FromSlash("../../testdata/schema.pb"))

	existing := exampleConfigMap()
	utils.SetMetaDataAnnotation(existing, "someanno", "origvalue")
	addOrigAnnotation(existing)
	utils.SetMetaDataAnnotation(existing, "someanno", "existingvalue")
	new := exampleConfigMap()
	utils.SetMetaDataAnnotation(new, "someanno", "newvalue")

	result, err := patch(existing, new, schemaResources.LookupResource(existing.GroupVersionKind()))
	if err != nil {
		t.Errorf("patch() returned error: %v", err)
	}

	// `new` should win conflicts

	t.Logf("existing: %#v", existing)
	t.Logf("result: %#v", result)
	value := result.GetAnnotations()["someanno"]
	if value != "newvalue" {
		t.Errorf("annotation was %q", value)
	}
}
