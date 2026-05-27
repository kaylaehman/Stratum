package docker

// Temporary: forces `go mod tidy` to resolve the Docker SDK and its transitive
// build deps into go.mod/go.sum before the client is written, so parallel agents
// don't race on dependency resolution. Replaced by client.go.
import (
	_ "github.com/docker/docker/client"
)
