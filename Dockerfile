# Builds a Docker image that allows you to run the ksonnet command
# line on a file in your local directory.
#
# USAGE: Define a function like `ksonnet` below, and then run:
#
#   `ksonnet <command> [options]`
#
# ksonnet() { docker run -it --rm   \
#     --volume "$PWD":/wd           \
#     --volume ~/.kube/:/root/.kube \
#     --workdir /wd                 \
#     ksonnet/ksonnet               \
#     kubecfg "$@"
# }

FROM golang:1.8
ENV KUBECFG_VERSION v0.5.0

RUN go get github.com/ksonnet/kubecfg
RUN cd /go/src/github.com/ksonnet/kubecfg && make && mv ./kubecfg /go/bin
WORKDIR /go/src/github.com/ksonnet/kubecfg
