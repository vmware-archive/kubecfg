// +build integration

package integration

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	// For client auth plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
var kubecfgBin = flag.String("kubecfg-bin", "kubecfg", "path to kubecfg executable under test")

func clusterConfigOrDie() *rest.Config {
	var config *rest.Config
	var err error

	if *kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		panic(err.Error())
	}

	return config
}

func createNsOrDie(c corev1.CoreV1Interface, ns string) string {
	result, err := c.Namespaces().Create(
		&v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ns,
			},
		})
	if err != nil {
		panic(err.Error())
	}
	name := result.GetName()
	fmt.Fprintf(GinkgoWriter, "Created namespace %s\n", name)
	return name
}

func deleteNsOrDie(c corev1.CoreV1Interface, ns string) {
	err := c.Namespaces().Delete(ns, &metav1.DeleteOptions{})
	if err != nil {
		panic(err.Error())
	}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func runKubecfgWith(flags []string, input []runtime.Object) error {
	return runKubecfgWithOutput(flags, input, GinkgoWriter)
}

func runKubecfgWithOutput(flags []string, input []runtime.Object, output io.Writer) error {
	tmpdir, err := ioutil.TempDir("", "kubecfg-testdata")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	fname := filepath.Join(tmpdir, "input.yaml")

	f, err := os.Create(fname)
	if err != nil {
		return err
	}
	enc := unstructured.JSONFallbackEncoder{serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}.LegacyCodec(
		v1.SchemeGroupVersion,
		appsv1.SchemeGroupVersion,
	)}

	if err := encodeTo(f, enc, input); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	args := []string{}
	if *kubeconfig != "" && !containsString(flags, "--kubeconfig") {
		args = append(args, "--kubeconfig", *kubeconfig)
	}
	args = append(args, flags...)
	args = append(args, fname)

	fmt.Fprintf(GinkgoWriter, "Running %q %q\n", *kubecfgBin, args)
	cmd := exec.Command(*kubecfgBin, args...)
	cmd.Stdout = output
	cmd.Stderr = output

	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func encodeTo(w io.Writer, enc runtime.Encoder, objs []runtime.Object) error {
	for _, o := range objs {
		buf, err := runtime.Encode(enc, o)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "---\n")
		_, err = w.Write(buf)
		if err != nil {
			return err
		}
	}
	return nil
}

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "kubecfg integration tests")
}
