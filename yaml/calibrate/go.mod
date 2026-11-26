module github.com/go-openapi/conformance-suites/yaml/calibrate

go 1.26.0

require github.com/go-openapi/conformance-suites v0.0.4

require github.com/go-openapi/conformance-suites/yaml/yamlgen v0.0.4

require github.com/go-openapi/conformance-suites/yaml/corpus v0.0.4

require (
	github.com/go-openapi/conformance-suites/yaml/consensus v0.0.4
	github.com/go-openapi/testify/v2 v2.8.1
	pgregory.net/rapid v1.3.0
)

require (
	github.com/go-openapi/core/json v0.0.3 // indirect
	github.com/go-openapi/swag/conv v0.28.0 // indirect
	github.com/go-openapi/swag/pools v0.28.0 // indirect
)

require (
	github.com/go-openapi/go-yaml v0.0.0-20261126182158-1aed24577d00
	go.yaml.in/yaml/v3 v3.0.6 // indirect
)

replace github.com/go-openapi/conformance-suites => ../..

replace github.com/go-openapi/conformance-suites/yaml/yamlgen => ../yamlgen

replace github.com/go-openapi/conformance-suites/yaml/corpus => ../corpus

replace github.com/go-openapi/conformance-suites/yaml/consensus => ../consensus
