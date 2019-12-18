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


// Example and reusable library to work with 1Password to store and retrieve passwords.
// Makes use of 'kubecfg.libsonnet' native functions:
// - generatePassword
// - execProgram
// - ntHashFromPassword

local kubecfg = import "kubecfg.libsonnet";
local lengthDefault = 16;
local numDigitsDefault = 4;
local numSymbolsDefault = 4;
local noUpperDefault = false;
local allowRepeatDefault = true;
local customSymbolsDefault = ""; // "" causes library to use the default symbol set

{
    generatePassword(field, length = lengthDefault, numDigits = numDigitsDefault, numSymbols = numSymbolsDefault,
                     noUpper = noUpperDefault, allowRepeat = allowRepeatDefault, customSymbols = customSymbolsDefault):: (
        kubecfg.generatePassword(length, numDigits, numSymbols, noUpper, allowRepeat, customSymbols)
    ),

    getPasswordFrom1Password(onePasswordItemName, vault):: (
        local item = $._get1PasswordItemByName(onePasswordItemName, vault);
        std.trace
            (if item == null 
                then "Could not retrieve item from 1Password (probably not signed-in)" 
                else "Item retrieved from 1Password for item '" + onePasswordItemName + "'", 
            if item == null then "N/A" else item.details.password)
    ),

    getItemFrom1Password(onePasswordItemName, vault):: (
        local item = $._get1PasswordItemByName(onePasswordItemName, vault);
        std.trace
            (if item == null 
                then "Could not retrieve item from 1Password (probably not signed-in)" 
                else "Item retrieved from 1Password for item '" + onePasswordItemName + "'", 
            item)
    ),

    ntHashFromPassword(password):: (
        kubecfg.ntHashFromPassword(password)
    ),

    _get1PasswordItemByName(name, vault):: (
        local itemString = kubecfg.execProgram("op", "get item " + name + " --vault=" + vault, false);
        if itemString == "" then null else std.parseJson(itemString)
    ),

    _generateSecrets(passwordsSpec):: { 
        [key]:  (
                local v = passwordsSpec[key];
                if std.type(v) == "object" then
                    // use object fields as params to generate a password
                    $.generatePassword(key, if "length" in v then v.length else lengthDefault, 
                                        if "numDigits" in v then v.numDigits else numDigitsDefault, 
                                        if "numSymbols" in v then v.numSymbols else numSymbolsDefault, 
                                        if "noUpper" in v then v.noUpper else noUpperDefault, 
                                        if "allowRepeat" in v then v.allowRepeat else allowRepeatDefault,
                                        "!@#$%^&*()_+`-={}|[]?,.",
                                        //"._+:@%/-", // reduced set, should be safe for passing in shell without escaping
                    )
                else 
                    // just use the value verbatim
                    v
                )
                for key in std.objectFields(passwordsSpec)
    },

    _saveTo1Password(name, vault, stringData):: (
        local item = {
            fields: [],
            sections: [
                {
                    name: "kubecfg",
                    title: "kubecfg",
                    fields: [
                        {
                            k: "concealed",
                            n: key,
                            t: key,
                            v: stringData[key],
                        },
                        for key in std.objectFields(stringData)
                    ]
                }
            ],
            passwordHistory: [],
            notesPlain: ""
        };

        // trigger side effect and return stringData
        local creationResult = $._createItemIn1Password(name, vault, item);
        std.trace(if creationResult == "" then "No item created in 1Password (probably not signed-in)" else "Created item in 1Password: " + creationResult, stringData)
    ),

    _createItemIn1Password(name, vault, item):: (
        local encodedItem = kubecfg.encodeBase64Url(std.toString(item));
        kubecfg.execProgram("op", "create item 'secure note' " + encodedItem + " --title=" + name + " --tags=iks,k8s,ibmcloud --vault=" + vault, false)
    ),

    _getKubecfgFieldsFrom1PasswordItem(onePasswordItem):: {
        ["fields"]: s.fields for s in onePasswordItem.details.sections if s.name == "kubecfg"
    },

    _convert1PasswordItemToSecrets(onePasswordItem):: {
        [f.n]: f.v for f in $._getKubecfgFieldsFrom1PasswordItem(onePasswordItem).fields
    },

    // This should be considered a workaround.
    //
    // Converts a Jsonnet object to string and back.
    // This is useful to make sure the new object only contains values and no references to functions
    // (e.g. functions like generatePassword that would otherwise be evaluated every time the field value is accessed)
    // see "Jsonnet Doesn't Re-use Intermediate Results" at https://databricks.com/blog/2018/10/12/writing-a-faster-jsonnet-compiler.html
    _serializeObject(object):: (
        std.parseJson(std.manifestJsonEx(object, "  "))
    ),

    OnePasswordSecret(name, namespace, vault): $._Object("v1", "Secret", name, namespace) {
        local onePasswordItemName = namespace + "-" + name,
        local onePasswordItem = $._get1PasswordItemByName(onePasswordItemName, vault),

        // Set the value of this field for the passwords that should be generated.
        // See 'secrets.jsonnet' example.
        generatedPasswords_:: {},
        
        type: "Opaque",
        stringData: if onePasswordItem == null 
                    then $._saveTo1Password(onePasswordItemName, vault, $._serializeObject($._generateSecrets(self.generatedPasswords_)))
                    else $._convert1PasswordItemToSecrets(onePasswordItem),
    },


    // The following is taken from https://github.com/bitnami-labs/kube-libsonnet/blob/master/kube.libsonnet
    // When kube.libsonnet is used 1PasswordSecret may reference it directly.
    safeName(s):: (
        local length = std.length(s);
        local name = (if length > 63 then std.substr(s, 0, 62) else s);
        std.asciiLower(std.join("-", std.split(name, ".")))
    ),

    _Object(apiVersion, kind, name, namespace = null):: {
        local this = self,
        apiVersion: apiVersion,
        kind: kind,
        metadata: {
        name: $.safeName(name),
        [if namespace != null then "namespace"]: namespace,
        labels: { name: std.join("-", std.split(this.metadata.name, ":")) },
        annotations: {},
        },
    },
}