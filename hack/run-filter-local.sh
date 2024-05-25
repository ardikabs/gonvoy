#!/bin/bash

filter=$1
with_build=${2-}

defaultGoImageVersion=golang:1.22-bullseye
defaultEnvoyImageVersion=envoyproxy/envoy:contrib-v1.30-latest

if [[ "${with_build:+yes}" == "yes" ]]; then
  docker run --rm \
    -v ".:/go/src/" \
    -w /go/src/ \
    -e "GOFLAGS=-buildvcs=false" \
    "${defaultGoImageVersion}" \
    bash -c "cd /go/src/test/e2e/filters/${filter} && go build -buildmode=c-shared -o filter.so *.go"
fi

docker run --rm \
  -p 8001:8000 -p 10001:10000 \
  -v "./test/e2e/filters/${filter}/envoy.yaml:/etc/envoy.yaml" \
  -v "./test/e2e/filters/${filter}/filter.so:/filter.so" \
  "${defaultEnvoyImageVersion}" \
  /usr/local/bin/envoy -c /etc/envoy.yaml \
  --log-level warn \
  --component-log-level http:info,golang:info,misc:error
