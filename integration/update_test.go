// +build integration

package integration

import (
	"bytes"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/bitnami/kubecfg/pkg/kubecfg"
	"github.com/bitnami/kubecfg/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func cmData(cm *v1.ConfigMap) map[string]string {
	return cm.Data
}

var _ = Describe("update", func() {
	var c corev1.CoreV1Interface
	var ns string
	const cmName = "testcm"

	BeforeEach(func() {
		c = corev1.NewForConfigOrDie(clusterConfigOrDie())
		ns = createNsOrDie(c, "update")
	})
	AfterEach(func() {
		deleteNsOrDie(c, ns)
	})

	Describe("An erroneous update", func() {
		var obj *unstructured.Unstructured
		var kubecfgErr error
		var kubecfgOut *bytes.Buffer
		BeforeEach(func() {
			obj = &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name": cmName,
					},
					"data": map[string]string{
						"foo": "bar",
					},
				},
			}
			kubecfgOut = &bytes.Buffer{}
		})

		JustBeforeEach(func() {
			kubecfgErr = runKubecfgWithOutput([]string{"update", "-vv", "-n", ns}, []runtime.Object{obj}, kubecfgOut)
		})

		Context("With invalid kind", func() {
			BeforeEach(func() {
				obj.SetKind("CanfogMop")
			})
			It("Should fail with a useful error", func() {
				Expect(kubecfgErr).To(HaveOccurred())
				Expect(kubecfgOut.String()).
					To(ContainSubstring(`"/v1, Kind=CanfogMop" not found`))
			})
		})

		Context("With superfluous json", func() {
			// NB: This would normally be silently ignored
			// by the apiserver, without explicit
			// client-side schema validation.
			BeforeEach(func() {
				obj.Object["extrakey"] = "extravalue"
			})
			It("Should fail with a useful error", func() {
				Expect(kubecfgErr).To(HaveOccurred())
				Expect(kubecfgOut.String()).
					To(ContainSubstring(`unknown field "extrakey" in io.k8s.api.core.v1.ConfigMap`))
			})
		})
	})

	Describe("A simple update", func() {
		var cm *v1.ConfigMap
		var kubecfgOut *bytes.Buffer
		BeforeEach(func() {
			cm = &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: cmName,
					Annotations: map[string]string{
						"my-annotation": "annotation-value",
					},
				},
				Data: map[string]string{"foo": "bar"},
			}
			kubecfgOut = &bytes.Buffer{}
		})

		JustBeforeEach(func() {
			err := runKubecfgWithOutput([]string{"update", "-vv", "-n", ns}, []runtime.Object{cm}, kubecfgOut)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("With no existing state", func() {
			It("should produce expected object", func() {
				Expect(c.ConfigMaps(ns).Get(cmName, metav1.GetOptions{})).
					To(WithTransform(cmData, HaveKeyWithValue("foo", "bar")))
				Expect(kubecfgOut.String()).
					To(ContainSubstring("Creating configmaps %s", cmName))
				Expect(kubecfgOut.String()).
					NotTo(ContainSubstring("Updating configmaps %s", cmName))
			})
		})

		Context("With existing non-kubecfg object", func() {
			BeforeEach(func() {
				_, err := c.ConfigMaps(ns).Create(cm)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should succeed", func() {
				Expect(c.ConfigMaps(ns).Get(cmName, metav1.GetOptions{})).
					To(WithTransform(cmData, HaveKeyWithValue("foo", "bar")))
				// NB: may report "Updating" - that's ok.
				Expect(kubecfgOut.String()).
					NotTo(ContainSubstring("Creating configmaps %s", cmName))
			})
		})

		Context("With no change", func() {
			BeforeEach(func() {
				err := runKubecfgWith([]string{"update", "-vv", "-n", ns}, []runtime.Object{cm})
				Expect(err).NotTo(HaveOccurred())

				Expect(c.ConfigMaps(ns).Get(cmName, metav1.GetOptions{})).
					To(WithTransform(metav1.Object.GetAnnotations, HaveKey(kubecfg.AnnotationOrigObject)))
			})

			It("should not report a change", func() {
				Expect(c.ConfigMaps(ns).Get(cmName, metav1.GetOptions{})).
					To(WithTransform(cmData, HaveKeyWithValue("foo", "bar")))
				// no change -> should report neither Updating nor Creating
				Expect(kubecfgOut.String()).
					NotTo(ContainSubstring("Updating configmaps %s", cmName))
				Expect(kubecfgOut.String()).
					NotTo(ContainSubstring("Creating configmaps %s", cmName))
			})
		})

		Context("With modified object", func() {
			BeforeEach(func() {
				otherCm := &v1.ConfigMap{
					ObjectMeta: cm.ObjectMeta,
					Data:       map[string]string{"foo": "not bar"},
				}

				err := runKubecfgWith([]string{"update", "-vv", "-n", ns}, []runtime.Object{otherCm})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should update the object", func() {
				Expect(c.ConfigMaps(ns).Get(cmName, metav1.GetOptions{})).
					To(WithTransform(cmData, HaveKeyWithValue("foo", "bar")))
				Expect(kubecfgOut.String()).
					To(ContainSubstring("Updating configmaps %s", cmName))
				Expect(kubecfgOut.String()).
					NotTo(ContainSubstring("Creating configmaps %s", cmName))
			})
		})

		Context("With externally modified object", func() {
			BeforeEach(func() {
				otherCm := &v1.ConfigMap{
					ObjectMeta: cm.ObjectMeta,
					Data:       map[string]string{"foo": "not bar"},
				}

				// Created by kubecfg ...
				err := runKubecfgWith([]string{"update", "-vv", "-n", ns}, []runtime.Object{otherCm})
				Expect(err).NotTo(HaveOccurred())

				// ... and then modified by another controller/whatever.
				o, err := c.ConfigMaps(ns).Patch(cmName, types.MergePatchType,
					[]byte(`{"metadata": {"annotations": {"addedby": "3rd-party"}}}`))
				Expect(err).NotTo(HaveOccurred())
				fmt.Fprintf(GinkgoWriter, "patch result is %v\n", o)
			})

			It("should update the object", func() {
				cm, err := c.ConfigMaps(ns).Get(cmName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(cm).To(WithTransform(cmData, HaveKeyWithValue("foo", "bar")))
				Expect(cm).To(WithTransform(metav1.Object.GetAnnotations, And(
					HaveKeyWithValue("addedby", "3rd-party"),
					HaveKeyWithValue("my-annotation", "annotation-value"),
				)))

				Expect(kubecfgOut.String()).
					To(ContainSubstring("Updating configmaps %s", cmName))
				Expect(kubecfgOut.String()).
					NotTo(ContainSubstring("Creating configmaps %s", cmName))
			})
		})
	})

	Describe("Update of existing object", func() {
		var objs []runtime.Object
		BeforeEach(func() {
			objs = []runtime.Object{}
		})
		JustBeforeEach(func() {
			err := runKubecfgWith([]string{"update", "--ignore-unknown", "-vv", "-n", ns}, objs)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("Custom type with no schema", func() {
			var obj *unstructured.Unstructured
			var rc dynamic.NamespaceableResourceInterface
			BeforeEach(func() {
				obj = &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "random.example.com/v1",
						"kind":       "Thing",
						"metadata": map[string]interface{}{
							"name": "thing1",
							"annotations": map[string]interface{}{
								"gen": "one",
							},
						},
					},
				}

				crd := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apiextensions.k8s.io/v1beta1",
						"kind":       "CustomResourceDefinition",
						"metadata": map[string]interface{}{
							"name": "things.random.example.com",
						},
						"spec": map[string]interface{}{
							// Note: no schema information
							"group":   "random.example.com",
							"version": "v1",
							"scope":   "Namespaced",
							"names": map[string]interface{}{
								"plural": "things",
								"kind":   "Thing",
							},
						},
					},
				}

				objs = append(objs, obj, crd)

				client, err := dynamic.NewForConfig(clusterConfigOrDie())
				Expect(err).NotTo(HaveOccurred())
				rc = client.Resource(schema.GroupVersionResource{
					Group:    "random.example.com",
					Version:  "v1",
					Resource: "things",
				})

			})
			AfterEach(func() {
				client, err := dynamic.NewForConfig(clusterConfigOrDie())
				Expect(err).NotTo(HaveOccurred())
				rc := client.Resource(schema.GroupVersionResource{
					Group:    "apiextensions.k8s.io",
					Version:  "v1beta1",
					Resource: "customresourcedefinitions",
				})
				_ = rc.Delete("things.random.example.com", &metav1.DeleteOptions{})
			})

			It("should update correctly", func() {
				Expect(rc.Namespace(ns).Get("thing1", metav1.GetOptions{})).
					To(WithTransform(metav1.Object.GetAnnotations,
						HaveKeyWithValue("gen", "one")))

				// Perform an update
				utils.SetMetaDataAnnotation(obj, "gen", "two")
				utils.SetMetaDataAnnotation(obj, "unrelated", "baz")
				err := runKubecfgWith([]string{"update", "-vv", "-n", ns}, objs)
				Expect(err).NotTo(HaveOccurred())

				// Verify result
				Expect(rc.Namespace(ns).Get("thing1", metav1.GetOptions{})).
					To(WithTransform(metav1.Object.GetAnnotations, And(
						HaveKeyWithValue("gen", "two"),
						HaveKeyWithValue("unrelated", "baz"),
					)))
			})
		})

		Context("Service type=NodePort", func() {
			// https://github.com/bitnami/kubecfg/issues/226

			const svcName = "example"
			BeforeEach(func() {
				objs = append(objs, &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name: svcName,
						Annotations: map[string]string{
							"generation": "one",
						},
					},
					Spec: v1.ServiceSpec{
						Ports: []v1.ServicePort{{
							Protocol: "TCP",
							Port:     80,
						}},
						Type: "NodePort",
					},
				})
			})

			It("should not change ports on subsequent updates", func() {
				svc, err := c.Services(ns).Get(svcName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(svc).To(WithTransform(metav1.Object.GetAnnotations,
					HaveKeyWithValue("generation", "one")))
				port := svc.Spec.Ports[0].NodePort
				Expect(port).To(BeNumerically(">", 0))

				// Perform an update
				utils.SetMetaDataAnnotation(objs[0].(*v1.Service), "generation", "two")
				err = runKubecfgWith([]string{"update", "-vv", "-n", ns}, objs)
				Expect(err).NotTo(HaveOccurred())

				// Check NodePort hasn't changed
				svc, err = c.Services(ns).Get(svcName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(svc).To(WithTransform(metav1.Object.GetAnnotations,
					HaveKeyWithValue("generation", "two")))
				Expect(svc.Spec.Ports[0].NodePort).
					To(Equal(port))
			})
		})

		Context("Container resources{}", func() {
			// https://github.com/ksonnet/kubecfg/issues/226

			const deployName = "example"
			BeforeEach(func() {
				objs = append(objs, &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: deployName,
						Annotations: map[string]string{
							"gen": "one",
						},
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"name": "c"},
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{"name": "c"},
							},
							Spec: v1.PodSpec{
								Containers: []v1.Container{{
									Name:  "c",
									Image: "k8s.gcr.io/pause",
								}},
							},
						},
					},
				})
			})

			It("should merge correctly into containers[]", func() {
				// Simulate change by 3rd-party
				appsc := appsv1client.NewForConfigOrDie(clusterConfigOrDie())
				_, err := appsc.Deployments(ns).Patch(deployName,
					types.StrategicMergePatchType,
					[]byte(`{"spec": {"template": {"spec": {"containers": [{"name": "c", "resources": {"limits": {"cpu": "1"}}}]}}}}`),
				)
				Expect(err).NotTo(HaveOccurred())

				// Perform an update
				utils.SetMetaDataAnnotation(objs[0].(*appsv1.Deployment), "gen", "two")
				err = runKubecfgWith([]string{"update", "-vv", "-n", ns}, objs)
				Expect(err).NotTo(HaveOccurred())

				// Check container.resources was preserved
				d, err := appsc.Deployments(ns).Get(deployName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(d).To(WithTransform(metav1.Object.GetAnnotations,
					HaveKeyWithValue("gen", "two")))
				Expect(d.Spec.Template.Spec.Containers[0].Resources.Limits).
					To(HaveKeyWithValue(v1.ResourceCPU, resource.MustParse("1")))

			})
		})
	})

	Describe("An update with mixed namespaces", func() {
		var ns2 string
		BeforeEach(func() {
			ns2 = createNsOrDie(c, "update")
		})
		AfterEach(func() {
			deleteNsOrDie(c, ns2)
		})

		var objs []runtime.Object
		BeforeEach(func() {
			objs = []runtime.Object{
				&v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "nons"},
				},
				&v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "ns1"},
				},
				&v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: ns2, Name: "ns2"},
				},
			}
		})

		JustBeforeEach(func() {
			err := runKubecfgWith([]string{"update", "-vv", "-n", ns}, objs)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create objects in the correct namespaces", func() {
			Expect(c.ConfigMaps(ns).Get("nons", metav1.GetOptions{})).
				NotTo(BeNil())

			Expect(c.ConfigMaps(ns).Get("ns1", metav1.GetOptions{})).
				NotTo(BeNil())

			Expect(c.ConfigMaps(ns2).Get("ns2", metav1.GetOptions{})).
				NotTo(BeNil())
		})
	})

	Context("With garbage collection enabled", func() {
		var preExist []*v1.ConfigMap
		var input []*v1.ConfigMap
		var gcTag string
		var dryRun bool
		var skipGc bool

		BeforeEach(func() {
			gcTag = "tag-" + ns
			preExist = []*v1.ConfigMap{}
			input = []*v1.ConfigMap{}
			dryRun = false
			skipGc = false
		})

		JustBeforeEach(func() {
			for _, obj := range preExist {
				_, err := c.ConfigMaps(ns).Create(obj)
				Expect(err).NotTo(HaveOccurred())
			}

			args := []string{"update", "-vv", "-n", ns}
			if gcTag != "" {
				args = append(args, "--gc-tag", gcTag)
			}
			if skipGc {
				args = append(args, "--skip-gc")
			}
			if dryRun {
				args = append(args, "--dry-run")
			}

			inputObjs := make([]runtime.Object, len(input))
			for i := range input {
				inputObjs[i] = input[i]
			}
			err := runKubecfgWith(args, inputObjs)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("With existing objects", func() {
			BeforeEach(func() {
				preExist = []*v1.ConfigMap{
					{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								// [gctag-migration]: Change to label-only in phase2
								kubecfg.AnnotationGcTag: gcTag,
							},
							Labels: map[string]string{
								kubecfg.LabelGcTag: gcTag,
							},
							Name: "existing",
						},
					},
					{
						// [gctag-migration]: Pre-migration test. Remove in phase2
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								kubecfg.AnnotationGcTag: gcTag,
							},
							// No LabelGcTag!
							Name: "existing-premigration",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								// [gctag-migration]: Change to label-only in phase2
								kubecfg.AnnotationGcTag: gcTag,
							},
							Labels: map[string]string{
								kubecfg.LabelGcTag: gcTag,
							},
							Name: "existing-stale",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "existing-stale-notag",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								// [gctag-migration]: Change to label-only in phase2
								kubecfg.AnnotationGcTag: gcTag + "-not",
							},
							Labels: map[string]string{
								kubecfg.LabelGcTag: gcTag + "-not",
							},
							Name: "existing-othertag",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								// [gctag-migration]: Change to label-only in phase2
								kubecfg.AnnotationGcTag:      gcTag,
								kubecfg.AnnotationGcStrategy: kubecfg.GcStrategyIgnore,
							},
							Labels: map[string]string{
								kubecfg.LabelGcTag: gcTag,
							},
							Name: "existing-precious",
						},
					},
				}

				input = []*v1.ConfigMap{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "new",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "existing",
						},
					},
					{
						// [gctag-migration]: Pre-migration test. Remove in phase2
						ObjectMeta: metav1.ObjectMeta{
							Name: "existing-premigration",
						},
					},
				}
			})

			It("should add gctag to new object", func() {
				o, err := c.ConfigMaps(ns).Get("new", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				// [gctag-migration]: Remove annotation in phase2
				Expect(o.ObjectMeta.Annotations).
					To(HaveKeyWithValue(kubecfg.AnnotationGcTag, gcTag))
				Expect(o.ObjectMeta.Labels).
					To(HaveKeyWithValue(kubecfg.LabelGcTag, gcTag))
			})

			It("should keep gctag on existing object", func() {
				o, err := c.ConfigMaps(ns).Get("existing", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				// [gctag-migration]: Remove annotation in phase2
				Expect(o.ObjectMeta.Annotations).
					To(HaveKeyWithValue(kubecfg.AnnotationGcTag, gcTag))
				Expect(o.ObjectMeta.Labels).
					To(HaveKeyWithValue(kubecfg.LabelGcTag, gcTag))
			})

			// [gctag-migration]: Pre-migration test. Remove in phase2
			It("should add gctag label to pre-migration object", func() {
				o, err := c.ConfigMaps(ns).Get("existing-premigration", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(o.ObjectMeta.Labels).
					To(HaveKeyWithValue(kubecfg.LabelGcTag, gcTag))
			})

			It("should delete stale object", func() {
				_, err := c.ConfigMaps(ns).Get("existing-stale", metav1.GetOptions{})
				Expect(errors.IsNotFound(err)).To(BeTrue())
			})

			It("should not delete tagless object", func() {
				Expect(c.ConfigMaps(ns).Get("existing-stale-notag", metav1.GetOptions{})).
					NotTo(BeNil())
			})

			It("should not delete object with different gc tag", func() {
				Expect(c.ConfigMaps(ns).Get("existing-othertag", metav1.GetOptions{})).
					NotTo(BeNil())
			})

			It("should not delete strategy=ignore object", func() {
				Expect(c.ConfigMaps(ns).Get("existing-precious", metav1.GetOptions{})).
					NotTo(BeNil())
			})
		})

		Context("with dry-run", func() {
			BeforeEach(func() {
				dryRun = true
				preExist = []*v1.ConfigMap{
					{
						ObjectMeta: metav1.ObjectMeta{
							// [gctag-migration]: Remove annotation in phase2
							Annotations: map[string]string{
								kubecfg.AnnotationGcTag: gcTag,
							},
							Labels: map[string]string{
								kubecfg.LabelGcTag: gcTag,
							},
							Name: "existing",
						},
					},
				}
				input = []*v1.ConfigMap{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "new",
						},
					},
				}
			})

			It("should not delete existing object", func() {
				Expect(c.ConfigMaps(ns).Get("existing", metav1.GetOptions{})).
					NotTo(BeNil())
			})

			It("should not create new object", func() {
				_, err := c.ConfigMaps(ns).Get("new", metav1.GetOptions{})
				Expect(errors.IsNotFound(err)).To(BeTrue())
			})
		})

		Context("with skip-gc", func() {
			BeforeEach(func() {
				skipGc = true
				preExist = []*v1.ConfigMap{
					{
						ObjectMeta: metav1.ObjectMeta{
							// [gctag-migration]: Remove annotation in phase2
							Annotations: map[string]string{
								kubecfg.AnnotationGcTag: gcTag,
							},
							Labels: map[string]string{
								kubecfg.LabelGcTag: gcTag,
							},
							Name: "existing",
						},
					},
				}
				input = []*v1.ConfigMap{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "new",
						},
					},
				}
			})

			It("should not delete existing object", func() {
				Expect(c.ConfigMaps(ns).Get("existing", metav1.GetOptions{})).
					NotTo(BeNil())
			})

			It("should add gctag to new object", func() {
				o, err := c.ConfigMaps(ns).Get("new", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				// [gctag-migration]: Remove annotation in phase2
				Expect(o.ObjectMeta.Annotations).
					To(HaveKeyWithValue(kubecfg.AnnotationGcTag, gcTag))
				Expect(o.ObjectMeta.Labels).
					To(HaveKeyWithValue(kubecfg.LabelGcTag, gcTag))
			})
		})
	})
})
