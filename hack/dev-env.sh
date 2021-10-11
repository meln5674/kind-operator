#!/bin/bash -xe

mkdir -p .tmp

docker build -f dev-env.Dockerfile .

docker build -q -f dev-env.Dockerfile . > .tmp/dev-env-image

docker run \
    --rm \
    -it \
    -v "$PWD:$PWD" \
    -v "$HOME:$HOME" \
    -v /etc/passwd:/etc/passwd:ro \
    -v /etc/group:/etc/group:ro \
    -e HOME \
    -e GOPATH \
    -w "$PWD" \
    -u "$(id -u):$(id -g)" \
    $(cat .tmp/dev-env-image)
