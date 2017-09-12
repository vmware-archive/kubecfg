## kubecfg prototype use

Instantiate prototype, emitting the generated code to stdout.

### Synopsis


Instantiate prototype uniquely identified by (possibly partial)
'prototype-name', filling in parameters from flags, and emitting the generated
code to stdout.

'prototype-name' need only contain enough of the suffix of a name to uniquely
disambiguate it among known names. For example, 'deployment' may resolve
ambiguously, in which case 'use' will fail, while 'simple-deployment' might be
unique enough to resolve to 'io.ksonnet.pkg.prototype.simple-deployment'.

```
kubecfg prototype use <prototype-name> [type] [parameter-flags]
```

### Examples

```
  # Instantiate prototype 'io.ksonnet.pkg.prototype.simple-deployment', using
  # the 'nginx' image, and port 80 exposed.
  ksonnet prototype use io.ksonnet.pkg.prototype.simple-deployment \
    --name=nginx                                                   \
    --image=nginx

  # Instantiate prototype using a unique suffix of an identifier. See
  # introduction of help message for more information on how this works.
  ksonnet prototype use simple-deployment \
    --name=nginx                          \
    --image=nginx
```

### Options inherited from parent commands

```
      --as string                      Username to impersonate for the operation
      --certificate-authority string   Path to a cert. file for the certificate authority
      --client-certificate string      Path to a client certificate file for TLS
      --client-key string              Path to a client key file for TLS
      --cluster string                 The name of the kubeconfig cluster to use
      --context string                 The name of the kubeconfig context to use
  -V, --ext-str stringSlice            Values of external variables
      --ext-str-file stringSlice       Read external variable from a file
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
  -J, --jpath stringSlice              Additional jsonnet library search path
      --kubeconfig string              Path to a kube config. Only required if out-of-cluster
  -n, --namespace string               If present, the namespace scope for this CLI request
      --password string                Password for basic authentication to the API server
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --resolve-images string          Change implementation of resolveImage native function. One of: noop, registry (default "noop")
      --resolve-images-error string    Action when resolveImage fails. One of ignore,warn,error (default "warn")
      --server string                  The address and port of the Kubernetes API server
  -A, --tla-str stringSlice            Values of top level arguments
      --tla-str-file stringSlice       Read top level argument from a file
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
      --username string                Username for basic authentication to the API server
  -v, --verbose count[=-1]             Increase verbosity. May be given multiple times.
```

### SEE ALSO
* [kubecfg prototype](kubecfg_prototype.md)	 - Instantiate, inspect, and get examples for ksonnet prototypes

