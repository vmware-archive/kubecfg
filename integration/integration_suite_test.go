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

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
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

func init() {
	registry := registered.NewOrDie(os.Getenv("KUBE_API_VERSIONS"))
	if missingVersions := registry.ValidateEnvRequestedVersions(); len(missingVersions) != 0 {
		panic(fmt.Sprintf("KUBE_API_VERSIONS contains versions that are not installed: %q.", missingVersions))
	}
	Install(make(announced.APIGroupFactoryRegistry), registry, runtime.NewScheme())
}

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
	enc := serializer.NewCodecFactory(runtime.NewScheme()).LegacyCodec(v1.SchemeGroupVersion)
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
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter

	if err := cmd.Run(); err != nil {
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

// Install registers the API group and adds types to a scheme
func Install(groupFactoryRegistry announced.APIGroupFactoryRegistry, registry *registered.APIRegistrationManager, scheme *runtime.Scheme) {
	if err := announced.NewGroupMetaFactory(
		&announced.GroupMetaFactoryArgs{
			GroupName:                  v1.GroupName,
			VersionPreferenceOrder:     []string{v1.SchemeGroupVersion.Version},
			AddInternalObjectsToScheme: v1.AddToScheme,
			RootScopedKinds: sets.NewString(
				"Node",
				"Namespace",
				"PersistentVolume",
				"ComponentStatus",
			),
			IgnoredKinds: sets.NewString(
				"ListOptions",
				"DeleteOptions",
				"Status",
				"PodLogOptions",
				"PodExecOptions",
				"PodAttachOptions",
				"PodPortForwardOptions",
				"PodProxyOptions",
				"NodeProxyOptions",
				"ServiceProxyOptions",
				"ThirdPartyResource",
				"ThirdPartyResourceData",
				"ThirdPartyResourceList",
			),
		},
		announced.VersionToSchemeFunc{
			v1.SchemeGroupVersion.Version: v1.AddToScheme,
		},
	).Announce(groupFactoryRegistry).RegisterAndEnable(registry, scheme); err != nil {
		panic(err)
	}
}
