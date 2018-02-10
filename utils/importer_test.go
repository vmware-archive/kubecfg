package utils

import (
	"net/url"
	"reflect"
	"testing"
)

func TestJoinURL(t *testing.T) {
	t.Run("file protocol join", func(t *testing.T) {
		u := url.URL{Scheme: "file", Host: "", Path: "/home/user/path/abc/cmd"}
		actual := joinURL(u, "../testdata/lib")
		expected := url.URL{Scheme: "file", Host: "", Path: "/home/user/path/abc/testdata/lib"}
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Expected %v, got %v", expected, actual)
		}
	})
	t.Run("http protocol join", func(t *testing.T) {
		u := url.URL{Scheme: "https", Host: "raw.githubusercontent.com", Path: "/ksonnet/ksonnet-lib/master"}
		actual := joinURL(u, "ksonnet.beta.2")
		expected := url.URL{Scheme: "https", Host: "raw.githubusercontent.com", Path: "/ksonnet/ksonnet-lib/master/ksonnet.beta.2"}
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Expected %v, got %v", expected, actual)
		}
	})
}

func TestDirURL(t *testing.T) {
	t.Run("Fixes up http scheme URLs", func(t *testing.T) {
		actual, _ := dirURL("http:/domain.com/path/abc")
		expected := url.URL{Scheme: "http", Host: "domain.com", Path: "/path/abc"}
		if !reflect.DeepEqual(*actual, expected) {
			t.Errorf("Expected %v, got %v", expected, actual)
		}
	})
	t.Run("Fixes up file scheme URLs", func(t *testing.T) {
		actual, _ := dirURL("file:/path/abc")
		expected := url.URL{Scheme: "file", Host: "", Path: "/path/abc"}
		if !reflect.DeepEqual(*actual, expected) {
			t.Errorf("Expected %v, got %v", expected, actual)
		}
	})

}

func TestExpandImportToCandidateURLs(t *testing.T) {
	importer := universalImporter{
		WorkDirURL: url.URL{Scheme: "file", Path: "/workdir/path"},
		BaseSearchURLs: []url.URL{
			url.URL{Scheme: "file", Path: "/first/base/search"},
		},
	}

	t.Run("Absolute URL in import statement yields a single candidate", func(t *testing.T) {
		urls, _ := importer.expandImportToCandidateURLs("dir", "http://absolute.com/import/path")
		expected := []url.URL{
			url.URL{Scheme: "http", Host: "absolute.com", Path: "/import/path"},
		}
		if !reflect.DeepEqual(urls, expected) {
			t.Errorf("Expected %v, got %v", expected, urls)
		}
	})

	t.Run("Absolute filesystem path in import statement yields a single candidate", func(t *testing.T) {
		urls, _ := importer.expandImportToCandidateURLs("dir", "/absolute/fs/path")
		expected := []url.URL{
			url.URL{Scheme: "file", Host: "", Path: "/absolute/fs/path"},
		}
		if !reflect.DeepEqual(urls, expected) {
			t.Errorf("Expected %v, got %v", expected, urls)
		}
	})

	t.Run("Absolute URL in import dir is searched before BaseSearchURLs", func(t *testing.T) {
		urls, _ := importer.expandImportToCandidateURLs("file:///abs/import/dir", "relative/file.libsonnet")
		expected := []url.URL{
			url.URL{Scheme: "file", Host: "", Path: "/abs/import/dir/relative/file.libsonnet"},
			url.URL{Scheme: "file", Host: "", Path: "/first/base/search/relative/file.libsonnet"},
		}
		if !reflect.DeepEqual(urls, expected) {
			t.Errorf("Expected %v, got %v", expected, urls)
		}
	})

	t.Run("Relative import is combined with workdir and search paths", func(t *testing.T) {
		urls, _ := importer.expandImportToCandidateURLs("../dir", "file.libsonnet")
		expected := []url.URL{
			url.URL{Scheme: "file", Host: "", Path: "/workdir/dir/file.libsonnet"},
			url.URL{Scheme: "file", Host: "", Path: "/first/base/search/file.libsonnet"},
		}
		if !reflect.DeepEqual(urls, expected) {
			t.Errorf("Expected %v, got %v", expected, urls)
		}
	})
}
