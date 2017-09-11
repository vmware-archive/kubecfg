// +build integration

package integration

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("show", func() {
	const ns = "showtest"
	var args []string
	var output []byte
	var input string
	var env []string

	BeforeEach(func() {
		args = []string{"show", "-vv", "-n", ns}
		if *kubeconfig != "" {
			args = append(args, "--kubeconfig", *kubeconfig)
		}

		env = os.Environ()
	})

	JustBeforeEach(func() {
		if input != "" {
			tmpdir, err := ioutil.TempDir("", "showtest")
			Expect(err).NotTo(HaveOccurred())
			fname := filepath.Join(tmpdir, "input.jsonnet")
			defer os.Remove(fname)

			err = ioutil.WriteFile(fname, []byte(input), 0666)
			Expect(err).NotTo(HaveOccurred())

			args = append(args, "-f", fname)
		}

		cmd := exec.Command(*kubecfgBin, args...)
		cmd.Env = env
		cmd.Stderr = GinkgoWriter

		var err error
		output, err = cmd.Output()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("with testdata input", func() {
		BeforeEach(func() {
			args = append(args,
				"-J", filepath.FromSlash("../testdata/lib"),
				"-f", filepath.FromSlash("../testdata/test.jsonnet"),
				"-V", "aVar=aVal",
				"-V", "anVar",
				"--ext-str-file", "filevar="+filepath.FromSlash("../testdata/extvar.file"),
			)

			env = append(env, "anVar=aVal2")
		})

		Context("with -o json", func() {
			BeforeEach(func() {
				args = append(args, "-o", "json")
			})

			It("should output expected JSON", func() {
				const expected = `
{
  "apiVersion": "v0alpha1",
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
}
`
				Expect(output).To(MatchJSON(expected))
			})
		})

		Context("with -o yaml", func() {
			BeforeEach(func() {
				args = append(args, "-o", "yaml")
			})

			It("should output expected YAML", func() {
				const expected = `
apiVersion: v0alpha1
kind: TestObject
nil: null
bool: true
number: 42
string: bar
notAVal: aVal
notAnotherVal: aVal2
filevar: "foo\n"
array: ["one", 2, [3]]
object: {"foo": "bar"}
`
				Expect(output).To(MatchYAML(expected))
			})
		})
	})
})
