// +build integration

package integration

import (
	"bytes"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"

	"github.com/ksonnet/kubecfg/pkg/kubecfg"
	"github.com/ksonnet/kubecfg/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func cmData(cm *v1.ConfigMap) map[string]string {
	return cm.Data
}

func restClientPool(conf *restclient.Config) (*utils.ClientPool, discovery.DiscoveryInterface, error) {
	disco, err := discovery.NewDiscoveryClientForConfig(conf)
	if err != nil {
		return nil, nil, err
	}

	discoCache := utils.NewMemcachedDiscoveryClient(disco)
	pathresolver := dynamic.LegacyAPIPathResolverFunc

	pool := utils.NewClientPool(conf, pathresolver)
	return pool, discoCache, nil
}

func restClientPoolOrDie(conf *restclient.Config) (*utils.ClientPool, discovery.DiscoveryInterface) {
	p, d, err := restClientPool(conf)
	if err != nil {
		panic(err.Error())
	}
	return p, d
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
		BeforeEach(func() {
			cm = &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: cmName},
				Data:       map[string]string{"foo": "bar"},
			}
		})

		JustBeforeEach(func() {
			err := runKubecfgWith([]string{"update", "-vv", "-n", ns}, []runtime.Object{cm})
			Expect(err).NotTo(HaveOccurred())
		})

		Context("With no existing state", func() {
			It("should produce expected object", func() {
				Expect(c.ConfigMaps(ns).Get("testcm", metav1.GetOptions{})).
					To(WithTransform(cmData, HaveKeyWithValue("foo", "bar")))
			})
		})

		Context("With existing object", func() {
			BeforeEach(func() {
				_, err := c.ConfigMaps(ns).Create(cm)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should succeed", func() {

				Expect(c.ConfigMaps(ns).Get("testcm", metav1.GetOptions{})).
					To(WithTransform(cmData, HaveKeyWithValue("foo", "bar")))
			})
		})

		Context("With modified object", func() {
			BeforeEach(func() {
				otherCm := &v1.ConfigMap{
					ObjectMeta: cm.ObjectMeta,
					Data:       map[string]string{"foo": "not bar"},
				}

				_, err := c.ConfigMaps(ns).Create(otherCm)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should update the object", func() {
				Expect(c.ConfigMaps(ns).Get("testcm", metav1.GetOptions{})).
					To(WithTransform(cmData, HaveKeyWithValue("foo", "bar")))
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
