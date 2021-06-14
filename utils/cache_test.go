package utils

import (
	"os"
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
