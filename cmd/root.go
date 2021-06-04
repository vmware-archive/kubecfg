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

package cmd

import (
	"bytes"
	"encoding/json"
	goflag "flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/genuinetools/reg/registry"

	jsonnet "github.com/google/go-jsonnet"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	"github.com/bitnami/kubecfg/utils"

	// Register auth plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const (
	flagVerbose     = "verbose"
	flagJpath       = "jpath"
	flagJUrl        = "jurl"
	flagExtVar      = "ext-str"
	flagExtVarFile  = "ext-str-file"
	flagExtCode     = "ext-code"
	flagExtCodeFile = "ext-code-file"
	flagTLAVar      = "tla-str"
	flagTLAVarFile  = "tla-str-file"
	flagTLACode     = "tla-code"
	flagTLACodeFile = "tla-code-file"
	flagResolver    = "resolve-images"
	flagResolvFail  = "resolve-images-error"
)

var clientConfig clientcmd.ClientConfig
var overrides clientcmd.ConfigOverrides

func init() {
	RootCmd.PersistentFlags().CountP(flagVerbose, "v", "Increase verbosity. May be given multiple times.")
	RootCmd.PersistentFlags().StringArrayP(flagJpath, "J", nil, "Additional Jsonnet library search path, appended to the ones in the KUBECFG_JPATH env var. May be repeated.")
	RootCmd.MarkPersistentFlagFilename(flagJpath)
	RootCmd.PersistentFlags().StringArrayP(flagJUrl, "U", nil, "Additional Jsonnet library search path given as a URL. May be repeated.")
	RootCmd.PersistentFlags().StringArrayP(flagExtVar, "V", nil, "Values of external variables with string values")
	RootCmd.PersistentFlags().StringArray(flagExtVarFile, nil, "Read external variables with string values from files")
	RootCmd.MarkPersistentFlagFilename(flagExtVarFile)
	RootCmd.PersistentFlags().StringArray(flagExtCode, nil, "Values of external variables with values supplied as Jsonnet code")
	RootCmd.PersistentFlags().StringArray(flagExtCodeFile, nil, "Read external variables with values supplied as Jsonnet code from files")
	RootCmd.MarkPersistentFlagFilename(flagExtCodeFile)
	RootCmd.PersistentFlags().StringArrayP(flagTLAVar, "A", nil, "Values of top level arguments with string values")
	RootCmd.PersistentFlags().StringArray(flagTLAVarFile, nil, "Read top level arguments with string values from files")
	RootCmd.MarkPersistentFlagFilename(flagTLAVarFile)
	RootCmd.PersistentFlags().StringArray(flagTLACode, nil, "Values of top level arguments with values supplied as Jsonnet code")
	RootCmd.PersistentFlags().StringArray(flagTLACodeFile, nil, "Read top level arguments with values supplied as Jsonnet code from files")
	RootCmd.MarkPersistentFlagFilename(flagTLACodeFile)
	RootCmd.PersistentFlags().String(flagResolver, "noop", "Change implementation of resolveImage native function. One of: noop, registry")
	RootCmd.PersistentFlags().String(flagResolvFail, "warn", "Action when resolveImage fails. One of ignore,warn,error")

	// The "usual" clientcmd/kubectl flags
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	kflags := clientcmd.RecommendedConfigOverrideFlags("")
	RootCmd.PersistentFlags().StringVar(&loadingRules.ExplicitPath, "kubeconfig", "", "Path to a kube config. Only required if out-of-cluster")
	RootCmd.MarkPersistentFlagFilename("kubeconfig")
	clientcmd.BindOverrideFlags(&overrides, RootCmd.PersistentFlags(), kflags)
	clientConfig = clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)
}

// RootCmd is the root of cobra subcommand tree
var RootCmd = &cobra.Command{
	Use:           "kubecfg",
	Short:         "Synchronise Kubernetes resources with config files",
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		goflag.CommandLine.Parse([]string{})
		flags := cmd.Flags()
		out := cmd.OutOrStderr()
		log.SetOutput(out)

		logFmt := NewLogFormatter(out)
		log.SetFormatter(logFmt)

		verbosity, err := flags.GetCount(flagVerbose)
		if err != nil {
			return err
		}
		log.SetLevel(logLevel(verbosity))

		// Ask me how much I love glog/klog's interface.
		logflags := goflag.NewFlagSet(os.Args[0], goflag.ExitOnError)
		klog.InitFlags(logflags)
		logflags.Set("logtostderr", "true")
		if verbosity >= 2 {
			// Semi-arbitrary mapping to klog level.
			logflags.Set("v", fmt.Sprintf("%d", verbosity*3))
		}

		return nil
	},
}

// clientConfig.Namespace() is broken in client-go 3.0:
// namespace in config erroneously overrides explicit --namespace
func defaultNamespace(c clientcmd.ClientConfig) (string, error) {
	if overrides.Context.Namespace != "" {
		return overrides.Context.Namespace, nil
	}
	ns, _, err := c.Namespace()
	return ns, err
}

func logLevel(verbosity int) log.Level {
	switch verbosity {
	case 0:
		return log.InfoLevel
	default:
		return log.DebugLevel
	}
}

type logFormatter struct {
	escapes  *terminal.EscapeCodes
	colorise bool
}

// NewLogFormatter creates a new log.Formatter customised for writer
func NewLogFormatter(out io.Writer) log.Formatter {
	var ret = logFormatter{}
	if f, ok := out.(*os.File); ok {
		ret.colorise = terminal.IsTerminal(int(f.Fd()))
		ret.escapes = terminal.NewTerminal(f, "").Escape
	}
	return &ret
}

func (f *logFormatter) levelEsc(level log.Level) []byte {
	switch level {
	case log.DebugLevel:
		return []byte{}
	case log.WarnLevel:
		return f.escapes.Yellow
	case log.ErrorLevel, log.FatalLevel, log.PanicLevel:
		return f.escapes.Red
	default:
		return f.escapes.Blue
	}
}

func (f *logFormatter) Format(e *log.Entry) ([]byte, error) {
	buf := bytes.Buffer{}
	if f.colorise {
		buf.Write(f.levelEsc(e.Level))
		fmt.Fprintf(&buf, "%-5s ", strings.ToUpper(e.Level.String()))
		buf.Write(f.escapes.Reset)
	}

	buf.WriteString(strings.TrimSpace(e.Message))
	buf.WriteString("\n")

	return buf.Bytes(), nil
}

// NB: `path` is assumed to be in native-OS path separator form
func dirURL(path string) *url.URL {
	path = filepath.ToSlash(path)
	if path[len(path)-1] != '/' {
		// trailing slash is important
		path = path + "/"
	}
	return &url.URL{Scheme: "file", Path: path}
}

// JsonnetVM constructs a new jsonnet.VM, according to command line
// flags
func JsonnetVM(cmd *cobra.Command) (*jsonnet.VM, error) {
	vm := jsonnet.MakeVM()
	flags := cmd.Flags()

	var searchUrls []*url.URL

	jpath := filepath.SplitList(os.Getenv("KUBECFG_JPATH"))

	jpathArgs, err := flags.GetStringArray(flagJpath)
	if err != nil {
		return nil, err
	}
	jpath = append(jpath, jpathArgs...)

	for _, p := range jpath {
		p, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		searchUrls = append(searchUrls, dirURL(p))
	}

	sURLs, err := flags.GetStringArray(flagJUrl)
	if err != nil {
		return nil, err
	}

	// Special URL scheme used to find embedded content
	sURLs = append(sURLs, "internal:///")

	for _, ustr := range sURLs {
		u, err := url.Parse(ustr)
		if err != nil {
			return nil, err
		}
		if u.Path[len(u.Path)-1] != '/' {
			u.Path = u.Path + "/"
		}
		searchUrls = append(searchUrls, u)
	}

	for _, u := range searchUrls {
		log.Debugln("Jsonnet search path:", u)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("Unable to determine current working directory: %v", err)
	}

	vm.Importer(utils.MakeUniversalImporter(searchUrls))

	for _, spec := range []struct {
		flagName string
		inject   func(string, string)
		isCode   bool
		fromFile bool
	}{
		{flagExtVar, vm.ExtVar, false, false},
		// Treat as code to evaluate "importstr":
		{flagExtVarFile, vm.ExtCode, false, true},
		{flagExtCode, vm.ExtCode, true, false},
		{flagExtCodeFile, vm.ExtCode, true, true},
		{flagTLAVar, vm.TLAVar, false, false},
		// Treat as code to evaluate "importstr":
		{flagTLAVarFile, vm.TLACode, false, true},
		{flagTLACode, vm.TLACode, true, false},
		{flagTLACodeFile, vm.TLACode, true, true},
	} {
		entries, err := flags.GetStringArray(spec.flagName)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			kv := strings.SplitN(entry, "=", 2)
			if spec.fromFile {
				if len(kv) != 2 {
					return nil, fmt.Errorf("Failed to parse %s: missing '=' in %s", spec.flagName, entry)
				}
				// Ensure that the import path we construct here is absolute, so that our Importer
				// won't try to glean from an extVar or TLA reference the context necessary to
				// resolve a relative path.
				path := kv[1]
				u, err := url.Parse(path)
				if err != nil {
					return nil, err
				}
				if u.Scheme == "" {
					if !filepath.IsAbs(u.Path) {
						u.Path = filepath.Join(cwd, u.Path)
					}
					u.Scheme = "file"
				}
				var imp string
				if spec.isCode {
					imp = "import"
				} else {
					imp = "importstr"
				}
				spec.inject(kv[0], fmt.Sprintf("%s @'%s'", imp, strings.ReplaceAll(u.String(), "'", "''")))
			} else {
				switch len(kv) {
				case 1:
					if v, present := os.LookupEnv(kv[0]); present {
						spec.inject(kv[0], v)
					} else {
						return nil, fmt.Errorf("Missing environment variable: %s", kv[0])
					}
				case 2:
					spec.inject(kv[0], kv[1])
				}
			}
		}
	}

	resolver, err := buildResolver(cmd)
	if err != nil {
		return nil, err
	}
	utils.RegisterNativeFuncs(vm, resolver)

	return vm, nil
}

func buildResolver(cmd *cobra.Command) (utils.Resolver, error) {
	flags := cmd.Flags()
	resolver, err := flags.GetString(flagResolver)
	if err != nil {
		return nil, err
	}
	failAction, err := flags.GetString(flagResolvFail)
	if err != nil {
		return nil, err
	}

	ret := resolverErrorWrapper{}

	switch failAction {
	case "ignore":
		ret.OnErr = func(error) error { return nil }
	case "warn":
		ret.OnErr = func(err error) error {
			log.Warning(err.Error())
			return nil
		}
	case "error":
		ret.OnErr = func(err error) error { return err }
	default:
		return nil, fmt.Errorf("Bad value for --%s: %s", flagResolvFail, failAction)
	}

	switch resolver {
	case "noop":
		ret.Inner = utils.NewIdentityResolver()
	case "registry":
		ret.Inner = utils.NewRegistryResolver(registry.Opt{})
	default:
		return nil, fmt.Errorf("Bad value for --%s: %s", flagResolver, resolver)
	}

	return &ret, nil
}

type resolverErrorWrapper struct {
	Inner utils.Resolver
	OnErr func(error) error
}

func (r *resolverErrorWrapper) Resolve(image *utils.ImageName) error {
	err := r.Inner.Resolve(image)
	if err != nil {
		err = r.OnErr(err)
	}
	return err
}

func readObjs(cmd *cobra.Command, paths []string) ([]*unstructured.Unstructured, error) {
	vm, err := JsonnetVM(cmd)
	if err != nil {
		return nil, err
	}

	res := []*unstructured.Unstructured{}
	for _, path := range paths {
		objs, err := utils.Read(vm, path)
		if err != nil {
			return nil, fmt.Errorf("Error reading %s: %v", path, err)
		}
		res = append(res, utils.FlattenToV1(objs)...)
	}
	return res, nil
}

// For debugging
func dumpJSON(v interface{}) string {
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err.Error()
	}
	return string(buf.Bytes())
}

func getDynamicClients(cmd *cobra.Command) (dynamic.Interface, meta.RESTMapper, discovery.DiscoveryInterface, error) {
	conf, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Unable to read kubectl config: %v", err)
	}

	disco, err := discovery.NewDiscoveryClientForConfig(conf)
	if err != nil {
		return nil, nil, nil, err
	}
	discoCache := utils.NewMemcachedDiscoveryClient(disco)

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoCache)

	cl, err := dynamic.NewForConfig(conf)
	if err != nil {
		return nil, nil, nil, err
	}

	return cl, mapper, discoCache, nil
}
