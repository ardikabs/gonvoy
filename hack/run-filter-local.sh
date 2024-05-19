#!/bin/bash

filter=$1
defaultEnvoyImageVersion=envoyproxy/envoy:contrib-v1.29-latest

docker run --rm \
  -p 8001:8000 -p 10001:10000 \
  -v "./test/e2e/filters/${filter}/envoy.yaml:/etc/envoy.yaml" \
  -v "./test/e2e/filters/${filter}/filter.so:/filter.so" \
  "${defaultEnvoyImageVersion}" \
  /usr/local/bin/envoy -c /etc/envoy.yaml \
  --log-level warn \
  --component-log-level main:critical,misc:critical,golang:info
