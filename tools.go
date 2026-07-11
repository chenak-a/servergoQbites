//go:build tools
// +build tools

// Package tools pins build-time tool dependencies (e.g. gqlgen code
// generation) so `go generate` / `go run github.com/99designs/gqlgen` resolve
// against the module's go.sum. Not compiled into the server binary.
package tools

import (
	_ "github.com/99designs/gqlgen"
)
