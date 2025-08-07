module github.com/neomody77/fake-compose

go 1.23.0

toolchain go1.23.11

require (
	github.com/docker/docker v20.10.27+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cobra v1.8.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/docker/distribution => github.com/distribution/distribution v2.8.2+incompatible

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/moby/term v0.5.2 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	gotest.tools/v3 v3.5.2 // indirect
)
