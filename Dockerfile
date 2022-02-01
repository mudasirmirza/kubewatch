FROM golang AS builder

RUN apt-get update && \
    apt-get install -y --no-install-recommends build-essential && \
    apt-get clean && \
    mkdir -p "$GOPATH/src/github.com/mudasirmirza/kubewatch"

ADD . "$GOPATH/src/github.com/mudasirmirza/kubewatch"

RUN cd "$GOPATH/src/github.com/mudasirmirza/kubewatch" && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a --installsuffix cgo --ldflags="-s" -o /kubewatch

FROM bitnami/minideb:stretch
RUN install_packages ca-certificates

COPY --from=builder /kubewatch /bin/kubewatch

ENTRYPOINT ["/bin/kubewatch"]
