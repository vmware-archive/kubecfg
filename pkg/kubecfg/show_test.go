package kubecfg

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// return sorted list of directory entries and the hash of their contents.
// Example:
//   ./foo/bar/baz.yaml:b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c
//   ./foo/zar/aaa.yaml:7d865e959b2466918c9863afca942d0fb89d7c9ac0c99bafc3749504ded97730
func dirDigests(dir string) (res []string, err error) {
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		hasher := sha256.New()
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, err := io.Copy(hasher, f); err != nil {
			return err
		}
		h := hex.EncodeToString(hasher.Sum(nil)[:])
		line := fmt.Sprintf("%s:%s", rel, h)
		res = append(res, line)
		return nil
	})
	return
}

func TestShowExport(t *testing.T) {
	testObjects := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"apiVersion": "tests/v1alpha1",
				"kind":       "Dummy",
				"metadata": map[string]interface{}{
					"name": "foo",
				},
			},
		},
		{
			Object: map[string]interface{}{
				"apiVersion": "tests/v1alpha1",
				"kind":       "Dummy",
				"metadata": map[string]interface{}{
					"name":      "bar",
					"namespace": "myns",
				},
			},
		},
	}

	testCases := []struct {
		testObjects []*unstructured.Unstructured
		format      string
		want        []string
	}{
		{
			testObjects,
			DefaultFileNameFormat,
			[]string{
				"tests-v1alpha1.Dummy-default.foo.yaml:7fb7dfcbf33096d74bd582cb8d827c17372625b412f3a022c1f849dd1fc5a70a",
				"tests-v1alpha1.Dummy-myns.bar.yaml:f0e39aa44d1e55fb8b06a05c97f6c2082e484c5a64bc9f766e26675346ca26ff",
			},
		},
		{
			testObjects,
			`{{default "default" .metadata.namespace}}/{{.apiVersion}}.{{.kind}}/{{.metadata.name}}`,
			[]string{
				"default/tests-v1alpha1.Dummy/foo.yaml:7fb7dfcbf33096d74bd582cb8d827c17372625b412f3a022c1f849dd1fc5a70a",
				"myns/tests-v1alpha1.Dummy/bar.yaml:f0e39aa44d1e55fb8b06a05c97f6c2082e484c5a64bc9f766e26675346ca26ff",
			},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			tmpdir, err := ioutil.TempDir("", "kubecfg-export-test")
			if err != nil {
				t.Error(t)
			}
			t.Cleanup(func() {
				os.RemoveAll(tmpdir)
			})

			c, err := NewShowCmd("yaml", tmpdir, tc.format)
			if err != nil {
				t.Error(t)
			}
			if err := c.Run(tc.testObjects, ioutil.Discard); err != nil {
				t.Error(err)
			}

			got, err := dirDigests(tmpdir)
			if err != nil {
				t.Error(t)
			}

			if want := tc.want; !reflect.DeepEqual(got, want) {
				t.Errorf("got: %q, want: %q", got, want)
			}
		})
	}
}

func TestShowExportNonEmpty(t *testing.T) {
	testObjects := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"apiVersion": "tests/v1alpha1",
				"kind":       "Dummy",
				"metadata": map[string]interface{}{
					"name": "foo",
				},
			},
		},
	}

	tmpdir, err := ioutil.TempDir("", "kubecfg-export-test")
	if err != nil {
		t.Error(t)
	}
	t.Cleanup(func() {
		os.RemoveAll(tmpdir)
	})

	c, err := NewShowCmd("yaml", tmpdir, DefaultFileNameFormat)
	if err != nil {
		t.Error(t)
	}
	if err := c.Run(testObjects, ioutil.Discard); err != nil {
		t.Error(err)
	}

	// running it a second time should return an error because the directory is not empty
	if err := c.Run(testObjects, ioutil.Discard); !strings.Contains(err.Error(), "is not empty") {
		t.Errorf("Expecting 'is not empty' error, got %v", err)
	}

}