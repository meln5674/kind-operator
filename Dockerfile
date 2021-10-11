
FROM alpine AS downloader

ARG OS=linux
ARG DOCKER_VERSION=20.10.9
ARG DOCKERARCH=x86_64

RUN apk add curl \
 && curl -vL https://download.docker.com/${OS}/static/stable/${DOCKERARCH}/docker-${DOCKER_VERSION}.tgz \
  | tar xz -C /tmp/ docker/docker \
 && chmod +x /tmp/docker/docker

ARG ARCH=amd64
ARG KIND_MIRROR=https://github.com/kubernetes-sigs/kind/releases/download
ARG KIND_VERSION=v0.11.1

RUN curl -vL ${KIND_MIRROR}/${KIND_VERSION}/kind-${OS}-${ARCH} \
  > /tmp/kind \
 && chmod +x /tmp/kind

RUN mkdir -p /tmp/tmp

FROM scratch

ARG OS=linux
ARG ARCH=amd64

COPY --from=downloader /tmp/docker/docker /docker
COPY --from=downloader /tmp/kind /kind
COPY --from=downloader /tmp/tmp /tmp
COPY --from=downloader /etc/ssl/cert.pem /etc/ssl/cert.pem
COPY bin/kind-operator-${OS}-${ARCH} /kind-operator

ENV PATH=/

ENTRYPOINT ["/kind-operator"]
CMD ["manager"]
