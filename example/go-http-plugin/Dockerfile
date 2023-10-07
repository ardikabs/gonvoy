FROM golang:1.20-bullseye AS builder
WORKDIR /app

COPY . .
RUN GOFLAGS=-buildvcs=false go build -o lib/go_http_plugin.so -buildmode=c-shared *.go

FROM envoyproxy/envoy:contrib-v1.27-latest

ENV DEBIAN_FRONTEND=noninteractive
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    rm -f /etc/apt/apt.conf.d/docker-clean \
    && echo 'Binary::apt::APT::Keep-Downloaded-Packages "true";' | tee /etc/apt/apt.conf.d/keep-cache \
    && apt-get -qq update -y \
    && apt-get -qq install --no-install-recommends -y curl
COPY --from=builder /app/lib/go_http_plugin.so /lib/go_http_plugin.so
CMD ["/usr/local/bin/envoy", "-c", "/etc/envoy.yaml"]