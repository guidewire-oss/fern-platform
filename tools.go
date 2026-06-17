//go:build tools
// +build tools

// Package tools imports various code generation tools
// required by the project but not used in the actual code.
package tools

import (
	_ "github.com/99designs/gqlgen"
	_ "github.com/99designs/gqlgen/graphql/introspection"
	// mockgen is run by `go generate ./...` in CI; pin it here so `go mod tidy`
	// keeps github.com/golang/mock in go.mod. Without this, tidy drops it (nothing
	// committed imports it — only the generated *_mock.go files do), and CI's
	// generated mocks then fail to load (golangci-lint: "no export data for gomock").
	_ "github.com/golang/mock/mockgen"
)
