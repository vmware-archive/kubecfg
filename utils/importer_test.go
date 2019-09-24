package utils

import (
	"fmt"
	"net/url"
	"os"
	"reflect"
	"testing"
)

func TestInternalFS(t *testing.T) {
	fs := newInternalFS("lib")
	if _, err := fs.Open("kubecfg.libsonnet"); err != nil {
		t.Errorf("opening kubecfg.libsonnet failed! %v", err)
	}
	if _, err := fs.Open("noexist"); !os.IsNotExist(err) {
		t.Errorf("Incorrect noexist error: %v", err)
	}
	if _, err := fs.Open("noexist/foo"); !os.IsNotExist(err) {
		t.Errorf("Incorrect noexist dir error: %v", err)
	}

	// This test really belongs somewhere else, but it's easiest
	// to do here.
	if _, err := fs.Open("kubecfg_test.jsonnet"); err == nil {
		t.Errorf("kubecfg_test.jsonnet should not have been embedded")
	}
}

func TestExpandImportToCandidateURLs(t *testing.T) {
	importer := universalImporter{
		BaseSearchURLs: []*url.URL{
			{Scheme: "file", Path: "/first/base/search/"},
		},
		extVarBaseURL: &url.URL{
			Scheme: "file",
			Path:   "/current/working/dir/",
		},
	}

	t.Run("Absolute URL in import statement yields a single candidate", func(t *testing.T) {
		urls, _ := importer.expandImportToCandidateURLs("dir", "http://absolute.com/import/path")
		expected := []*url.URL{
			{Scheme: "http", Host: "absolute.com", Path: "/import/path"},
		}
		if !reflect.DeepEqual(urls, expected) {
			t.Errorf("Expected %v, got %v", expected, urls)
		}
	})

	t.Run("Absolute URL in import dir is searched before BaseSearchURLs", func(t *testing.T) {
		urls, _ := importer.expandImportToCandidateURLs("file:///abs/import/dir/", "relative/file.libsonnet")
		expected := []*url.URL{
			{Scheme: "file", Host: "", Path: "/abs/import/dir/relative/file.libsonnet"},
			{Scheme: "file", Host: "", Path: "/first/base/search/relative/file.libsonnet"},
		}
		if !reflect.DeepEqual(urls, expected) {
			t.Errorf("Expected %v, got %v", expected, urls)
		}
	})

	for _, test := range []struct {
		description string
		varKind     string
	}{
		{"external variable", "extvar"},
		{"top-level variable", "top-level-arg"},
	} {
		t.Run(fmt.Sprintf("Relative URL in import statement used as %s yields candidate relative to base URL", test.description), func(t *testing.T) {
			urls, _ := importer.expandImportToCandidateURLs(fmt.Sprintf("<%s:example>", test.varKind), "../sought.jsonnet")
			expected := []*url.URL{
				{Scheme: "file", Host: "", Path: "/current/working/sought.jsonnet"},
				{Scheme: "file", Host: "", Path: "/first/base/sought.jsonnet"},
			}
			if !reflect.DeepEqual(urls, expected) {
				t.Errorf("Expected %v, got %v", expected, urls)
			}
		})
	}
}
