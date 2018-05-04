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

// Run me with `../kubecfg show kubecfg_test.jsonnet`
local kubecfg = import "kubecfg.libsonnet";

local result =

std.assertEqual(kubecfg.parseJson("[3, 4]"), [3, 4]) &&

std.assertEqual(kubecfg.parseYaml(|||
                                    ---
                                    - 3
                                    - 4
                                    ---
                                    foo: bar
                                    baz: xyzzy
                                  ||| ),
                [[3, 4], {foo: "bar", baz: "xyzzy"}]) &&

std.assertEqual(kubecfg.manifestJson({foo: "bar", baz: [3, 4]}),
                |||
                  {
                      "baz": [
                          3,
                          4
                      ],
                      "foo": "bar"
                  }
               |||
               ) &&

std.assertEqual(kubecfg.manifestJson({foo: "bar", baz: [3, 4]}, indent=2),
                |||
                  {
                    "baz": [
                      3,
                      4
                    ],
                    "foo": "bar"
                  }
                |||
               ) &&

std.assertEqual(kubecfg.manifestJson("foo"), '"foo"\n') &&

std.assertEqual(kubecfg.manifestYaml({foo: "bar", baz: [3, 4]}),
                |||
                  baz:
                  - 3
                  - 4
                  foo: bar
                |||
               ) &&

std.assertEqual(kubecfg.resolveImage("busybox"),
                "busybox:latest") &&

std.assertEqual(kubecfg.regexMatch("o$", "foo"), true) &&

std.assertEqual(kubecfg.escapeStringRegex("f[o"), "f\\[o") &&

std.assertEqual(kubecfg.regexSubst("e", "tree", "oll"),
                "trolloll") &&

true;

// Kubecfg wants to see something that looks like a k8s object
{
  apiVersion: "test",
  kind: "Result",
  // result==false assert-aborts above, but we should use the value
  // somewhere here to ensure the expression actually gets evaluated.
  result: if result then "SUCCESS" else "FAILED",
}
