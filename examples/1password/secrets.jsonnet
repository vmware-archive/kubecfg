// Copyright 2019 The kubecfg authors
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

// Run make kubecfg
// Run ./kubecfg show examples/1password/secrets.jsonnet

local kubecfg = import "kubecfg.libsonnet";
local onePassword = import "1password.libsonnet";

function(namespace="test", useFallbackValues = false) {
    local clusterName = "eip3",

    // this is determining the item name (in 1Password) based on a name and namespace 
    // and the vault name based on a cluster name
    secrets: onePassword.OnePasswordSecret("secrets", namespace, "cluster-" + clusterName, useFallbackValues) {
        local secret = self,

        stringData+: {
          // passwords can be supplied verbatim - these will not be saved in 1Password
          "verbatim-unsaved": "pass123!@#$%^&*()_+`-={}|[]?,.",
          
          // passwords can be generated directly using a function - these will not be saved in 1Password
          "generated-with-kubecfg-libsonnet": kubecfg.generatePassword(8, 2, 2, false, true, ""),
          
          // passwords can be retrieved from a 1Password 'password' item - these will not be saved in 1Password since they already are
          "existing-password-stored-in-one-password": onePassword.getPasswordFrom1Password("secret-password", "cluster-" + clusterName),
          
          // a fallback value can be supplied in case the vault should not be accessed at all
          "existing-password-stored-in-one-password-with-fallback": onePassword.getPasswordFrom1Password("secret-password", "cluster-" + clusterName, "fallback-value-for-secret-password", useFallbackValues),

          // items can be retrieved verbatim from a 1Password item - these will not be saved in 1Password since they already are
          [if !useFallbackValues then "existing-item-stored-in-one-password"]: onePassword.getItemFrom1Password("secret-item", "cluster-" + clusterName).details.fields[1].value,
          
          // a fallback value (an object) can be supplied in case the vault should not be accessed at all
          "existing-item-stored-in-one-password-with-fallback": onePassword.getItemFrom1Password("secret-item", "cluster-" + clusterName, 
            {
              details: {
                fields: [
                  {
                    value: "fallback-user-value",
                  },
                  {
                    value: "fallback-password-value",
                  },
                ],
              },
            },
            useFallbackValues).details.fields[1].value,

          // a password hash can be calculated from a password
          // the password to use can be a generated one (see below) - these will not be saved in 1Password (they are recalculated every time)
          "nthash-1": kubecfg.ntHashFromPassword(secret.stringData["password-1-generated-with-spec"]),
          "nthash-2": onePassword.ntHashFromPassword(secret.stringData["password-2-generated-with-spec"])
        },
        
        // passwords can be generated (with different options applied when generating) - these will be saved in 1Password
        generatedPasswords_+: {
          "password-1-generated-with-spec": {
            length: 16,
            numDigits: 4,
            numSymbols: 6,
            noUpper: false,
            allowRepeat: true,
            fallback: "abc",
          },
          "password-2-generated-with-spec": {
            length: 32,
            numDigits: 1,
            numSymbols: 6,
            noUpper: false,
            allowRepeat: false,
            fallback: "def",
          },
          // defaults will be applied when no explicit values are supplied
          "password-3-generated-with-spec": {
            length: 64,
          },
          // defaults will be applied when no explicit values are supplied.
          "password-4-generated-with-spec": {},

          // passwords can be supplied verbatim - these will be saved in 1Password
          "verbatim-saved": "pass456!@#$%^&*()_+`-={}|[]?,.",
        },

    },
}
