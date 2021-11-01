// Copyright 2017 The kubecfg authors
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package kubecfg

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	yaml "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	DefaultFileNameFormat = `{{.apiVersion}}.{{.kind}}-{{default "default" .metadata.namespace}}.{{.metadata.name}}`

	// belRune is a string of the Ascii character BEL which made computers ring in ancient times
	// We use it as "magic" char for the subfolder creation as it is a non printable character and thereby will never be
	// in a valid filepath by accident. Only when we include it.
	//
	// Borrowed from tanka.
	belRune = string(rune(7))

	// Replace path separators "/" with this char
	pathSepReplacement = "-"
)

// ShowCmd represents the show subcommand
type ShowCmd struct {
	OutputFormat     string
	ExportDir        string
	fileNameTemplate *template.Template
}

func NewShowCmd(outputFormat, exportDir, fileNameFormat string) (ShowCmd, error) {
	replacedFileNameFormat := replaceTmplText(fileNameFormat, string(os.PathSeparator), belRune)
	fileNameTemplate, err := template.New("").Funcs(sprig.TxtFuncMap()).Parse(replacedFileNameFormat)
	if err != nil {
		return ShowCmd{}, err
	}
	return ShowCmd{
		OutputFormat:     outputFormat,
		ExportDir:        exportDir,
		fileNameTemplate: fileNameTemplate,
	}, nil
}

// from tanka
func replaceTmplText(s, old, new string) string {
	parts := []string{}
	l := strings.Index(s, "{{")
	r := strings.Index(s, "}}") + 2

	for l != -1 && l < r {
		// replace only in text between template action blocks
		text := strings.ReplaceAll(s[:l], old, new)
		action := s[l:r]
		parts = append(parts, text, action)
		s = s[r:]
		l = strings.Index(s, "{{")
		r = strings.Index(s, "}}") + 2
	}
	parts = append(parts, strings.ReplaceAll(s, old, new))
	return strings.Join(parts, "")
}

func (c ShowCmd) Run(apiObjects []*unstructured.Unstructured, out io.Writer) error {
	if c.ExportDir != "" {
		if err := os.MkdirAll(c.ExportDir, 0777); err != nil {
			return err
		}
		empty, err := isDirEmpty(c.ExportDir)
		if err != nil {
			return err
		}
		if !empty {
			return fmt.Errorf("export directory %q is not empty", c.ExportDir)
		}
	}

	for i, obj := range apiObjects {
		if err := c.renderObject(i, obj, out); err != nil {
			return err
		}
	}
	return nil
}

func (c ShowCmd) renderObject(idx int, obj *unstructured.Unstructured, out io.Writer) error {
	if c.ExportDir != "" {
		name, err := c.formatObjectName(idx, obj)
		if err != nil {
			return err
		}
		filename := fmt.Sprintf("%s.%s", name, c.OutputFormat)
		path := filepath.Join(c.ExportDir, filename)
		if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
			return err
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}

	switch c.OutputFormat {
	case "yaml":
		fmt.Fprintln(out, "---")
		o, err := k8sToJSONObject(obj)
		if err != nil {
			return err
		}
		buf, err := yaml.Marshal(o)
		if err != nil {
			return err
		}
		if _, err := out.Write(buf); err != nil {
			return err
		}
	case "json":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		// TODO: this is not valid framing for JSON
		if idx > 0 {
			fmt.Fprintln(out, "---")
		}
		if err := enc.Encode(obj); err != nil {
			return err
		}
	default:
		return fmt.Errorf("Unknown --format: %s", c.OutputFormat)
	}
	return nil
}

func k8sToJSONObject(obj *unstructured.Unstructured) (map[string]interface{}, error) {
	// Urgh.  Go via json because we need
	// to trigger the custom scheme
	// encoding.
	//
	// Otherwise we could just return obj.Object
	// encoding.
	buf, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	o := map[string]interface{}{}
	if err := json.Unmarshal(buf, &o); err != nil {
		return nil, err
	}
	return o, nil
}

func (c ShowCmd) formatObjectName(idx int, obj *unstructured.Unstructured) (string, error) {
	var buf strings.Builder

	// suboptimal: this is the easiest way to let the template function access fields in Unstructured
	o, err := k8sToJSONObject(obj)
	if err != nil {
		return "", err
	}
	if err := c.fileNameTemplate.Execute(&buf, o); err != nil {
		return "", err
	}

	// Replace all os.path separators in string in order to not accidentally create subfolders
	path := strings.Replace(buf.String(), string(os.PathSeparator), pathSepReplacement, -1)
	// Replace the BEL character inserted with a path separator again in order to create a subfolder
	path = strings.Replace(path, belRune, string(os.PathSeparator), -1)

	return path, nil
}

func isDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}
