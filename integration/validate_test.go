// +build integration

package integration

import (
	"bytes"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("validate", func() {
	var output string
	var input []runtime.Object
	var args []string
	var kubecfgErr error

	BeforeEach(func() {
		input = make([]runtime.Object, 0, 2)
		args = []string{"validate", "-vv"}
	})

	JustBeforeEach(func() {
		outbuf := bytes.Buffer{}
		kubecfgErr = runKubecfgWithOutput(args, input, &outbuf)
		output = outbuf.String()
	})

	Context("With valid input", func() {
		BeforeEach(func() {
			input = append(input, &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
				Data:       map[string]string{"foo": "bar"},
			})

			// CRDs frequently behave specially :(
			input = append(input, &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apiextensions.k8s.io/v1beta1",
					"kind":       "CustomResourceDefinition",
					"metadata": map[string]interface{}{
						"name": "crds.example.com",
					},
					"spec": map[string]interface{}{
						"group":   "example.com",
						"version": "v1alpha1",
						"names": map[string]string{
							"plural":   "crds",
							"singular": "crd",
							"kind":     "Crd",
						},
					},
				},
			})
		})

		It("should succeed", func() {
			Expect(kubecfgErr).NotTo(HaveOccurred())
			Expect(output).NotTo(ContainSubstring("Validation failed"))
		})
	})

	Context("With superfluous json", func() {
		BeforeEach(func() {
			input = append(input, &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":     "foo",
						"nimspace": "bar",
					},
				},
			})
		})

		It("Should fail with a useful error", func() {
			Expect(kubecfgErr).To(HaveOccurred())
			Expect(output).
				To(ContainSubstring(`unknown field "nimspace" in io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta`))
			Expect(output).
				To(ContainSubstring("Validation failed"))
		})
	})

	Context("With an unknown Kind", func() {
		BeforeEach(func() {
			input = append(input, &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "xConfigMap",
					"metadata": map[string]interface{}{
						"name": "foo",
					},
				},
			})
		})

		It("should succeed", func() {
			Expect(kubecfgErr).NotTo(HaveOccurred())
			Expect(output).NotTo(ContainSubstring("Validation failed"))
		})

		Context("With --ignore-unknown=false", func() {
			BeforeEach(func() {
				args = append(args, "--ignore-unknown=false")
			})

			It("Should fail with a useful error", func() {
				Expect(kubecfgErr).To(HaveOccurred())
				Expect(output).
					To(ContainSubstring(`"/v1, Kind=xConfigMap" not found`))
				Expect(output).
					To(ContainSubstring("Validation failed"))
			})
		})
	})

	Context("With an unknown API group", func() {
		BeforeEach(func() {
			input = append(input, &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.com/v1alpha1",
					"kind":       "UnregisteredKind",
					"metadata": map[string]interface{}{
						"name": "foo",
					},
					"data": map[string]string{
						"foo": "bar",
					},
				},
			})
		})

		It("should succeed", func() {
			Expect(kubecfgErr).NotTo(HaveOccurred())
			Expect(output).NotTo(ContainSubstring("Validation failed"))
		})

		Context("With --ignore-unknown=false", func() {
			BeforeEach(func() {
				args = append(args, "--ignore-unknown=false")
			})

			It("Should fail with a useful error", func() {
				Expect(kubecfgErr).To(HaveOccurred())
				Expect(output).
					To(ContainSubstring(`"example.com/v1alpha1, Kind=UnregisteredKind" not found`))
				Expect(output).
					To(ContainSubstring("Validation failed"))
			})
		})
	})
})
