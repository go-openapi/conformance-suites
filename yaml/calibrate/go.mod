module github.com/go-openapi/conformance-suites/yaml/calibrate

go 1.26.0

require github.com/go-openapi/conformance-suites v0.0.0

require github.com/go-openapi/conformance-suites/yaml/yamlgen v0.0.0

require github.com/go-openapi/conformance-suites/yaml/corpus v0.0.0

require (
	github.com/go-openapi/conformance-suites/yaml/consensus v0.0.0
	github.com/go-openapi/testify/v2 v2.8.1
	pgregory.net/rapid v1.3.0
)

require (
	github.com/go-openapi/go-yaml v0.0.0-20261122211517-e3188123aaae
	go.yaml.in/yaml/v3 v3.0.6 // indirect
)

replace github.com/go-openapi/conformance-suites => ../..

replace github.com/go-openapi/conformance-suites/yaml/yamlgen => ../yamlgen

replace github.com/go-openapi/conformance-suites/yaml/corpus => ../corpus

replace github.com/go-openapi/conformance-suites/yaml/consensus => ../consensus
