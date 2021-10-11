ARG GO_VERSION=1.17

FROM golang:${GO_VERSION}

ARG KUBEBUILDER_VERSION=3.1.0

RUN curl -vL "https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${KUBEBUILDER_VERSION}/kubebuilder_$(go env GOOS)_$(go env GOARCH)" -o /usr/local/bin/kubebuilder

ARG K8S_VERSION=1.19.2

RUN curl -sSL "https://storage.googleapis.com/kubebuilder-tools/kubebuilder-tools-${K8S_VERSION}-$(go env GOOS)-$(go env GOARCH).tar.gz" \
     | tar -xz --strip-components=1 -C /usr/local/bin/

RUN chmod -R +x /usr/local/bin/

ARG COBRA_VERSION=1.2.1

RUN go install github.com/spf13/cobra/cobra@v${COBRA_VERSION}

RUN find / -name cobra && which cobra

ENV PATH="${PATH}:/usr/local/bin:/go/bin"

RUN apt-get update && apt-get install -y vim
