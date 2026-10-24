module github.com/ardikabs/gonvoy

go 1.26

require (
	github.com/cncf/xds/go v0.0.0-20260202195803-dba9d589def2
	github.com/envoyproxy/envoy v1.40.0
	github.com/go-logr/logr v1.4.4
	github.com/rs/zerolog v1.35.1
	github.com/stretchr/testify v1.12.1
	github.com/tidwall/gjson v1.20.0
	google.golang.org/protobuf v1.36.12
	k8s.io/apimachinery v0.30.14
)

require (
	cel.dev/expr v0.25.1 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.3.3 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sys v0.47.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
)

retract v0.3.2
