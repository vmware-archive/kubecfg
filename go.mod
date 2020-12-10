module github.com/bitnami/kubecfg

require (
	github.com/Azure/go-autorest/autorest v0.10.0 // indirect
	github.com/elazarl/go-bindata-assetfs v1.0.1-0.20180223160309-38087fe4dafb
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/genuinetools/reg v0.16.1
	github.com/ghodss/yaml v1.0.0
	github.com/go-openapi/spec v0.19.7 // indirect
	github.com/go-openapi/swag v0.19.8 // indirect
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/golang/protobuf v1.3.5
	github.com/google/go-cmp v0.4.0 // indirect
	github.com/google/go-jsonnet v0.17.0
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/googleapis/gnostic v0.4.0
	github.com/imdario/mergo v0.3.9 // indirect
	github.com/json-iterator/go v1.1.9 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/kr/pretty v0.2.0 // indirect
	github.com/mailru/easyjson v0.7.1 // indirect
	github.com/mattn/go-isatty v0.0.11
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.7.0
	github.com/pkg/errors v0.9.1 // indirect
	github.com/sergi/go-diff v1.1.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.7
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.4.0
	golang.org/x/crypto v0.0.0-20200406173513-056763e48d71
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d // indirect
	golang.org/x/sys v0.0.0-20200409092240-59c9f1ba88fa // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	google.golang.org/appengine v1.6.5 // indirect
	google.golang.org/genproto v0.0.0-20200409111301-baae70f3302d // indirect
	google.golang.org/grpc v1.28.1 // indirect
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.17.4
	k8s.io/apiextensions-apiserver v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v0.17.4
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20191107075043-30be4d16710a
	k8s.io/kubernetes v1.13.4
	k8s.io/utils v0.0.0-20200327001022-6496210b90e8 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)

// NB: pinning gnostic to v0.4.0 as v0.4.1 renamed s/OpenAPIv2/openapiv2/ at
//       https://github.com/googleapis/gnostic/pull/155,
//     while k8s.io/client-go/discovery@v0.17 still uses OpenAPIv2,
//     even as of 2020/04/09 there's no released k8s.io/client-go/discovery version
//     (latest 0.18.1) fixed to use openapiv2
replace github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.0

go 1.13
