FROM golang:1.13 AS build
ARG VERSION


WORKDIR /go/src/kubecfg
COPY . .

RUN make

FROM gcr.io/distroless/base
COPY --from=build /go/src/kubecfg/kubecfg /
CMD ["/kubecfg"]
