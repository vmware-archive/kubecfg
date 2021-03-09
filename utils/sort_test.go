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
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	openapi_v2 "github.com/googleapis/gnostic/openapiv2"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	fakedisco "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/restmapper"
	ktesting "k8s.io/client-go/testing"
)

type FakeDiscovery struct {
	fakedisco.FakeDiscovery
	schemaGetter discovery.OpenAPISchemaInterface
}

func NewFakeDiscovery(schemaGetter discovery.OpenAPISchemaInterface) *FakeDiscovery {
	fakePtr := &ktesting.Fake{}
	return &FakeDiscovery{
		FakeDiscovery: fakedisco.FakeDiscovery{Fake: fakePtr},
		schemaGetter:  schemaGetter,
	}
}

func (c *FakeDiscovery) OpenAPISchema() (*openapi_v2.Document, error) {
	action := ktesting.ActionImpl{}
	action.Verb = "get"
	c.Fake.Invokes(action, nil)

	return c.schemaGetter.OpenAPISchema()
}

func TestDepSort(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	disco := NewFakeDiscovery(schemaFromFile{dir: filepath.FromSlash("../testdata")})
	disco.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "configmaps",
					Kind:       "ConfigMap",
					Namespaced: true,
				},
				{
					Name:       "namespaces",
					Kind:       "Namespace",
					Namespaced: false,
				},
				{
					Name:       "replicationcontrollers",
					Kind:       "ReplicationController",
					Namespaced: true,
				},
			},
		},
	}

	mapper := restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{{
		Group: metav1.APIGroup{
			Name: "",
			Versions: []metav1.GroupVersionForDiscovery{{
				GroupVersion: "v1",
				Version:      "v1",
			}},
		},
		VersionedResources: map[string][]metav1.APIResource{
			"v1": disco.Resources[0].APIResources,
		},
	}})

	newObj := func(apiVersion, kind string) *unstructured.Unstructured {
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": apiVersion,
				"kind":       kind,
			},
		}
	}

	objs := []*unstructured.Unstructured{
		newObj("v1", "ReplicationController"),
		newObj("v1", "ConfigMap"),
		newObj("v1", "Namespace"),
		newObj("admissionregistration.k8s.io/v1beta1", "MutatingWebhookConfiguration"),
		newObj("bogus/v1", "UnknownKind"),
		newObj("apiextensions.k8s.io/v1beta1", "CustomResourceDefinition"),
	}

	sorter, err := DependencyOrder(disco, mapper, objs)
	if err != nil {
		t.Fatalf("DependencyOrder error: %v", err)
	}
	sort.Sort(sorter)

	for i, o := range objs {
		t.Logf("obj[%d] after sort is %v", i, o.GroupVersionKind())
	}

	if objs[0].GetKind() != "CustomResourceDefinition" {
		t.Error("CRD should be sorted first")
	}
	if objs[1].GetKind() != "Namespace" {
		t.Error("Namespace should be sorted second")
	}
	if objs[4].GetKind() != "ReplicationController" {
		t.Error("RC should be sorted after non-pod objects")
	}
	if objs[5].GetKind() != "MutatingWebhookConfiguration" {
		t.Error("Webhook should be sorted last")
	}
}

func TestAlphaSort(t *testing.T) {
	newObj := func(ns, name, kind string) *unstructured.Unstructured {
		o := unstructured.Unstructured{}
		o.SetNamespace(ns)
		o.SetName(name)
		o.SetKind(kind)
		return &o
	}

	objs := []*unstructured.Unstructured{
		newObj("default", "mysvc", "Deployment"),
		newObj("", "default", "StorageClass"),
		newObj("", "default", "ClusterRole"),
		newObj("default", "mydeploy", "Deployment"),
		newObj("default", "mysvc", "Secret"),
	}

	expected := []*unstructured.Unstructured{
		objs[2],
		objs[1],
		objs[3],
		objs[0],
		objs[4],
	}

	sort.Sort(AlphabeticalOrder(objs))

	if !reflect.DeepEqual(objs, expected) {
		t.Errorf("actual != expected: %v != %v", objs, expected)
	}
}
