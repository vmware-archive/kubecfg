load("@io_bazel_rules_go//go:def.bzl", "gazelle", "go_binary", "go_library", "go_prefix")

gazelle(
    name = "gazelle",
    external = "vendored",
    prefix = "github.com/ksonnet/kubecfg",
)

go_prefix("github.com/ksonnet/kubecfg")

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importpath = "github.com/ksonnet/kubecfg",
    visibility = ["//visibility:private"],
    deps = [
        "//cmd:go_default_library",
        "//pkg/kubecfg:go_default_library",
        "//vendor/github.com/sirupsen/logrus:go_default_library",
    ],
)

go_binary(
    name = "kubecfg",
    importpath = "github.com/ksonnet/kubecfg",
    library = ":go_default_library",
    visibility = ["//visibility:public"],
)
