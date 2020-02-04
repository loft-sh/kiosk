# Build the manager binary
FROM golang:1.13 as builder

WORKDIR /workspace

# Install Helm 3
RUN bash -c "curl -s https://get.helm.sh/helm-v3.0.3-linux-amd64.tar.gz > helm3.tar.gz" && tar -zxvf helm3.tar.gz linux-amd64/helm && chmod +x linux-amd64/helm && mv linux-amd64/helm /workspace/helm && rm helm3.tar.gz && rm -R linux-amd64

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
COPY vendor/ vendor/

# Copy the go source
COPY cmd/manager/main.go cmd/manager/main.go
COPY cmd/apiserver/main.go cmd/apiserver/main.go
COPY pkg/ pkg/


ENV GO111MODULE on
ENV DEBUG true

# Build manager
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -mod vendor -o manager cmd/manager/main.go

# Build api server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -mod vendor -o apiserver cmd/apiserver/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/apiserver .
COPY --from=builder /workspace/helm .
USER nonroot:nonroot

# No entrypoint we do this in the kube yamls
