# Build the manager binary
FROM golang:1.16 as builder

WORKDIR /workspace
ARG TARGETOS=linux
ARG TARGETARCH=amd64

# Install Helm 3
RUN bash -c "curl -s https://get.helm.sh/helm-v3.8.0-linux-${TARGETARCH}.tar.gz > helm3.tar.gz" && tar -zxvf helm3.tar.gz linux-${TARGETARCH}/helm && chmod +x linux-${TARGETARCH}/helm && mv linux-${TARGETARCH}/helm /workspace/helm && rm helm3.tar.gz && rm -R linux-${TARGETARCH}

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
COPY vendor/ vendor/

# Copy the go source
COPY cmd/ cmd/
COPY kube/ kube/
COPY pkg/ pkg/

ENV GO111MODULE on
ENV DEBUG true

# Build kiosk
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GO111MODULE=on go build -mod vendor -o kiosk cmd/kiosk/main.go

# Use distroless as minimal base image to package the kiosk binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static
WORKDIR /
COPY --from=builder /workspace/kiosk .
COPY --from=builder /workspace/helm .

ENTRYPOINT ["/kiosk"]