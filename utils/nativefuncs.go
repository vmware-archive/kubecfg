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

package utils

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"os/exec"
	"strings"

	"github.com/DaKnOb/ntlm"
	"github.com/mattn/go-shellwords"
	"github.com/sethvargo/go-password/password"

	goyaml "github.com/ghodss/yaml"

	jsonnet "github.com/google/go-jsonnet"
	jsonnetAst "github.com/google/go-jsonnet/ast"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func resolveImage(resolver Resolver, image string) (string, error) {
	n, err := ParseImageName(image)
	if err != nil {
		return "", err
	}

	if err := resolver.Resolve(&n); err != nil {
		return "", err
	}

	return n.String(), nil
}

func generatePassword(length int, numDigits int, numSymbols int, noUpper bool, allowRepeat bool, customSymbols string) (string, error) {
	// exclude some of the default symbols as they cause problems when using them
	// as arguments on the command line (as part of JSON being passed)
	input := &password.GeneratorInput {
		LowerLetters: "",
		UpperLetters: "",
		Digits:       "",
		Symbols:      customSymbols,
	}
	g, err := password.NewGenerator(input)

	if err != nil {
		return "", err
	}
	return g.Generate(length, numDigits, numSymbols, noUpper, allowRepeat)
}

func execProgram(name string, argumentsString string, failOnError bool) (string, error) {
	arg, err := shellwords.Parse(argumentsString)
	if err != nil {
		if failOnError {
			return "", err
		}
		return "", nil
	}

	out, err := exec.Command(name, arg...).CombinedOutput()
	if err != nil {
		if failOnError {
			return "", errors.New(string(out))
		}
		return "", nil
	}

	return string(out), nil
}

func ntHashFromPassword(password string) (string)  {
	return string(ntlm.FromASCIIStringToHex(password))
}

func encodeBase64Url(text string) (string) {
	return base64.URLEncoding.EncodeToString([]byte(text))
}

// RegisterNativeFuncs adds kubecfg's native jsonnet functions to provided VM
func RegisterNativeFuncs(vm *jsonnet.VM, resolver Resolver) {
	// TODO(mkm): go-jsonnet 0.12.x now contains native std.parseJson; deprecate and remove this one.
	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "parseJson",
		Params: []jsonnetAst.Identifier{"json"},
		Func: func(args []interface{}) (res interface{}, err error) {
			data := []byte(args[0].(string))
			err = json.Unmarshal(data, &res)
			return
		},
	})

	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "parseYaml",
		Params: []jsonnetAst.Identifier{"yaml"},
		Func: func(args []interface{}) (res interface{}, err error) {
			ret := []interface{}{}
			data := []byte(args[0].(string))
			d := yaml.NewYAMLToJSONDecoder(bytes.NewReader(data))
			for {
				var doc interface{}
				if err := d.Decode(&doc); err != nil {
					if err == io.EOF {
						break
					}
					return nil, err
				}
				ret = append(ret, doc)
			}
			return ret, nil
		},
	})

	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "manifestJson",
		Params: []jsonnetAst.Identifier{"json", "indent"},
		Func: func(args []interface{}) (res interface{}, err error) {
			value := args[0]
			indent := int(args[1].(float64))
			data, err := json.MarshalIndent(value, "", strings.Repeat(" ", indent))
			if err != nil {
				return "", err
			}
			data = append(data, byte('\n'))
			return string(data), nil
		},
	})

	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "manifestYaml",
		Params: []jsonnetAst.Identifier{"json"},
		Func: func(args []interface{}) (res interface{}, err error) {
			value := args[0]
			output, err := goyaml.Marshal(value)
			return string(output), err
		},
	})

	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "resolveImage",
		Params: []jsonnetAst.Identifier{"image"},
		Func: func(args []interface{}) (res interface{}, err error) {
			return resolveImage(resolver, args[0].(string))
		},
	})

	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "escapeStringRegex",
		Params: []jsonnetAst.Identifier{"str"},
		Func: func(args []interface{}) (res interface{}, err error) {
			return regexp.QuoteMeta(args[0].(string)), nil
		},
	})

	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "regexMatch",
		Params: []jsonnetAst.Identifier{"regex", "string"},
		Func: func(args []interface{}) (res interface{}, err error) {
			return regexp.MatchString(args[0].(string), args[1].(string))
		},
	})

	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "regexSubst",
		Params: []jsonnetAst.Identifier{"regex", "src", "repl"},
		Func: func(args []interface{}) (res interface{}, err error) {
			regex := args[0].(string)
			src := args[1].(string)
			repl := args[2].(string)

			r, err := regexp.Compile(regex)
			if err != nil {
				return "", err
			}
			return r.ReplaceAllString(src, repl), nil
		},
	})

	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "generatePassword",
		Params: []jsonnetAst.Identifier{"length", "numDigits", "numSymbols", "noUpper", "allowRepeat", "customSymbols"},
		Func: func(args []interface{}) (res interface{}, err error) {
			return generatePassword(int(args[0].(float64)), int(args[1].(float64)), int(args[2].(float64)), args[3].(bool), args[4].(bool), args[5].(string) )
		},
	})

	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "execProgram",
		Params: []jsonnetAst.Identifier{"name", "arguments", "failOnError"},
		Func: func(args []interface{}) (res interface{}, err error) {
			return execProgram(args[0].(string), args[1].(string), args[2].(bool))
		},
	})
	
	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "ntHashFromPassword",
		Params: []jsonnetAst.Identifier{"password"},
		Func: func(args []interface{}) (res interface{}, err error) {
			return ntHashFromPassword(args[0].(string)), nil
		},
	})

	vm.NativeFunction(&jsonnet.NativeFunction{
		Name:   "encodeBase64Url",
		Params: []jsonnetAst.Identifier{"text"},
		Func: func(args []interface{}) (res interface{}, err error) {
			return encodeBase64Url(args[0].(string)), nil
		},
	})

}
