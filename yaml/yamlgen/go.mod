module github.com/go-openapi/conformance-suites/yaml/yamlgen

go 1.26.0

require github.com/go-openapi/conformance-suites v0.0.0

require (
	github.com/go-openapi/conformance-suites/yaml/corpus v0.0.0
	github.com/go-openapi/testify/v2 v2.8.1
	pgregory.net/rapid v1.3.0
)

replace github.com/go-openapi/conformance-suites => ../..

replace github.com/go-openapi/conformance-suites/yaml/corpus => ../corpus
