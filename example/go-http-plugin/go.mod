module go-http-plugin

go 1.20

require (
	github.com/ardikabs/go-envoy v0.0.0-20231009053952-ace845d7b847
	github.com/cncf/xds/go v0.0.0-20231016030527-8bd2eac9fb4a
	github.com/envoyproxy/envoy v1.28.0
	google.golang.org/protobuf v1.31.0
)

require (
	github.com/envoyproxy/protoc-gen-validate v1.0.2 // indirect
	github.com/go-logr/logr v1.3.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/rs/zerolog v1.31.0 // indirect
	golang.org/x/sys v0.14.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20231106174013-bbf56f31fb17 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231106174013-bbf56f31fb17 // indirect
)

replace github.com/ardikabs/go-envoy => ../../
