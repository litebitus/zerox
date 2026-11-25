module github.com/litebitus/zerox

go 1.27.4

require (
	github.com/litebitus/yaml-dsl-spec/go v0.3.4
	github.com/litebitus/zerox/client/go v0.0.0
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.1.0
	go.yaml.in/yaml/v4 v4.0.0
)

require (
	github.com/dlclark/regexp2 v1.12.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace github.com/litebitus/zerox/client/go => ./client/go
