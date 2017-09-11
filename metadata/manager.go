package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/spf13/afero"

	"github.com/ksonnet/ksonnet-lib/ksonnet-gen/ksonnet"
	"github.com/ksonnet/ksonnet-lib/ksonnet-gen/kubespec"
)

func appendToAbsPath(originalPath AbsPath, toAppend ...string) AbsPath {
	paths := append([]string{string(originalPath)}, toAppend...)
	return AbsPath(path.Join(paths...))
}

const (
	ksonnetDir      = ".ksonnet"
	libDir          = "lib"
	componentsDir   = "components"
	environmentsDir = "environments"
	vendorDir       = "vendor"

	defaultEnvName = "default"

	// Environment-specific files
	schemaFilename        = "swagger.json"
	extensionsLibFilename = "k.libsonnet"
	k8sLibFilename        = "k8s.libsonnet"
	specFilename          = "spec.json"
)

type manager struct {
	appFS afero.Fs

	rootPath        AbsPath
	ksonnetPath     AbsPath
	libPath         AbsPath
	componentsPath  AbsPath
	environmentsDir AbsPath
	vendorDir       AbsPath
}

type environmentSpec struct {
	URI string `json:"uri"`
}

func findManager(abs AbsPath, appFS afero.Fs) (*manager, error) {
	var lastBase string
	currBase := string(abs)

	for {
		currPath := path.Join(currBase, ksonnetDir)
		exists, err := afero.Exists(appFS, currPath)
		if err != nil {
			return nil, err
		}
		if exists {
			return newManager(AbsPath(currBase), appFS), nil
		}

		lastBase = currBase
		currBase = filepath.Dir(currBase)
		if lastBase == currBase {
			return nil, fmt.Errorf("No ksonnet application found")
		}
	}
}

func initManager(rootPath AbsPath, spec ClusterSpec, appFS afero.Fs) (*manager, error) {
	m := newManager(rootPath, appFS)

	// Generate the program text for ksonnet-lib.
	//
	// IMPLEMENTATION NOTE: We get the cluster specification and generate
	// ksonnet-lib before initializing the directory structure so that failure of
	// either (e.g., GET'ing the spec from a live cluster returns 404) does not
	// result in a partially-initialized directory structure.
	//
	extensionsLibData, k8sLibData, err := m.GenerateKsonnetLibData(spec)
	if err != nil {
		return nil, err
	}

	// Initialize directory structure.
	if err := m.createAppDirTree(); err != nil {
		return nil, err
	}

	// Initialize environment, and cache specification data.
	// TODO the URI for the default environment needs to be generated from KUBECONFIG
	if err := m.CreateEnvironment(defaultEnvName, "", spec, extensionsLibData, k8sLibData); err != nil {
		return nil, err
	}

	return m, nil
}

func newManager(rootPath AbsPath, appFS afero.Fs) *manager {
	return &manager{
		appFS: appFS,

		rootPath:        rootPath,
		ksonnetPath:     appendToAbsPath(rootPath, ksonnetDir),
		libPath:         appendToAbsPath(rootPath, libDir),
		componentsPath:  appendToAbsPath(rootPath, componentsDir),
		environmentsDir: appendToAbsPath(rootPath, environmentsDir),
		vendorDir:       appendToAbsPath(rootPath, vendorDir),
	}
}

func (m *manager) Root() AbsPath {
	return m.rootPath
}

func (m *manager) ComponentPaths() (AbsPaths, error) {
	paths := AbsPaths{}
	err := afero.Walk(m.appFS, string(m.componentsPath), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return paths, nil
}

func (m *manager) LibPaths(envName string) (libPath, envLibPath AbsPath) {
	return m.libPath, appendToAbsPath(m.environmentsDir, envName)
}

func (m *manager) CreateEnvironment(name, uri string, spec ClusterSpec, extensionsLibData, k8sLibData []byte) error {
	envPath := appendToAbsPath(m.environmentsDir, name)
	err := m.appFS.MkdirAll(string(envPath), os.ModePerm)
	if err != nil {
		return err
	}

	// Get cluster specification data, possibly from the network.
	specData, err := spec.data()
	if err != nil {
		return err
	}

	// Generate the schema file.
	schemaPath := appendToAbsPath(envPath, schemaFilename)
	err = afero.WriteFile(m.appFS, string(schemaPath), specData, os.ModePerm)
	if err != nil {
		return err
	}

	k8sLibPath := appendToAbsPath(envPath, k8sLibFilename)
	err = afero.WriteFile(m.appFS, string(k8sLibPath), k8sLibData, 0644)
	if err != nil {
		return err
	}

	extensionsLibPath := appendToAbsPath(envPath, extensionsLibFilename)
	err = afero.WriteFile(m.appFS, string(extensionsLibPath), extensionsLibData, 0644)
	if err != nil {
		return err
	}

	// Generate the environment spec file.
	envSpecData, err := generateEnvironmentSpecData(uri)
	if err != nil {
		return err
	}

	envSpecPath := appendToAbsPath(envPath, specFilename)
	return afero.WriteFile(m.appFS, string(envSpecPath), envSpecData, os.ModePerm)
}

func (m *manager) GenerateKsonnetLibData(spec ClusterSpec) ([]byte, []byte, error) {
	// Get cluster specification data, possibly from the network.
	text, err := spec.data()
	if err != nil {
		return nil, nil, err
	}

	ksonnetLibDir := appendToAbsPath(m.environmentsDir, defaultEnvName)

	// Deserialize the API object.
	s := kubespec.APISpec{}
	err = json.Unmarshal(text, &s)
	if err != nil {
		return nil, nil, err
	}

	s.Text = text
	s.FilePath = filepath.Dir(string(ksonnetLibDir))

	// Emit Jsonnet code.
	return ksonnet.Emit(&s, nil, nil)
}

func (m *manager) createAppDirTree() error {
	exists, err := afero.DirExists(m.appFS, string(m.rootPath))
	if err != nil {
		return fmt.Errorf("Could not check existance of directory '%s':\n%v", m.rootPath, err)
	} else if exists {
		return fmt.Errorf("Could not create app; directory '%s' already exists", m.rootPath)
	}

	paths := []AbsPath{
		m.rootPath,
		m.ksonnetPath,
		m.libPath,
		m.componentsPath,
		m.vendorDir,
	}

	for _, p := range paths {
		if err := m.appFS.MkdirAll(string(p), os.ModePerm); err != nil {
			return err
		}
	}

	return nil
}

func generateEnvironmentSpecData(uri string) ([]byte, error) {
	// Format the spec json and return; preface keys with 2 space idents.
	return json.MarshalIndent(environmentSpec{URI: uri}, "", "  ")
}
