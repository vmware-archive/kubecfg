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
	"fmt"
	"sync"

	"github.com/googleapis/gnostic/OpenAPIv2"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/kubectl/cmd/util/openapi"
)

type memcachedDiscoveryClient struct {
	cl              discovery.DiscoveryInterface
	lock            sync.RWMutex
	servergroups    *metav1.APIGroupList
	serverresources map[string]*metav1.APIResourceList
	schemas         map[string]openapi.Resources
	schema          *openapi_v2.Document
}

// NewMemcachedDiscoveryClient creates a new DiscoveryClient that
// caches results in memory
func NewMemcachedDiscoveryClient(cl discovery.DiscoveryInterface) discovery.CachedDiscoveryInterface {
	c := &memcachedDiscoveryClient{cl: cl}
	c.Invalidate()
	return c
}

func (c *memcachedDiscoveryClient) Fresh() bool {
	return true
}

func (c *memcachedDiscoveryClient) Invalidate() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.servergroups = nil
	c.serverresources = make(map[string]*metav1.APIResourceList)
	c.schemas = make(map[string]openapi.Resources)
}

func (c *memcachedDiscoveryClient) RESTClient() rest.Interface {
	return c.cl.RESTClient()
}

func (c *memcachedDiscoveryClient) ServerGroups() (*metav1.APIGroupList, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	var err error
	if c.servergroups != nil {
		return c.servergroups, nil
	}
	c.servergroups, err = c.cl.ServerGroups()
	return c.servergroups, err
}

func (c *memcachedDiscoveryClient) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	var err error
	if v := c.serverresources[groupVersion]; v != nil {
		return v, nil
	}
	c.serverresources[groupVersion], err = c.cl.ServerResourcesForGroupVersion(groupVersion)
	return c.serverresources[groupVersion], err
}

func (c *memcachedDiscoveryClient) ServerResources() ([]*metav1.APIResourceList, error) {
	return discovery.ServerResources(c)
}

func (c *memcachedDiscoveryClient) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return discovery.ServerPreferredResources(c)
}

func (c *memcachedDiscoveryClient) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return discovery.ServerPreferredNamespacedResources(c)
}

func (c *memcachedDiscoveryClient) ServerVersion() (*version.Info, error) {
	return c.cl.ServerVersion()
}

func (c *memcachedDiscoveryClient) OpenAPISchema() (*openapi_v2.Document, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.schema != nil {
		return c.schema, nil
	}

	schema, err := c.cl.OpenAPISchema()
	if err != nil {
		return nil, err
	}

	c.schema = schema
	return schema, nil
}

var _ discovery.CachedDiscoveryInterface = &memcachedDiscoveryClient{}

// ClientForResource returns the ResourceClient for a given object
func ClientForResource(client dynamic.Interface, mapper meta.RESTMapper, obj runtime.Object, defNs string) (dynamic.ResourceInterface, error) {
	gvk := obj.GetObjectKind().GroupVersionKind()

	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	rc := client.Resource(mapping.Resource)

	switch mapping.Scope.Name() {
	case meta.RESTScopeNameRoot:
		return rc, nil
	case meta.RESTScopeNameNamespace:
		meta, err := meta.Accessor(obj)
		if err != nil {
			return nil, err
		}
		namespace := meta.GetNamespace()
		if namespace == "" {
			namespace = defNs
		}
		return rc.Namespace(namespace), nil
	default:
		return nil, fmt.Errorf("unexpected resource scope %q", mapping.Scope)
	}
}
