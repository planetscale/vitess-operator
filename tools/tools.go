// +build tools

package tools

// These imports ensure that "go mod tidy" won't remove deps
// for build-time dependencies like linters and code generators.
import (
	_ "github.com/ahmetb/gen-crd-api-reference-docs"
	_ "github.com/operator-framework/operator-sdk/cmd/operator-sdk"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
