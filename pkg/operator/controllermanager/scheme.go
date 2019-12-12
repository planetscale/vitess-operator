package controllermanager

import (
	"k8s.io/apimachinery/pkg/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	appsv1defaults "k8s.io/kubernetes/pkg/apis/apps/v1"
	batchv1defaults "k8s.io/kubernetes/pkg/apis/batch/v1"
	corev1defaults "k8s.io/kubernetes/pkg/apis/core/v1"

	planetscaleapis "planetscale.dev/vitess-operator/pkg/apis"
)

// schemeAddFuncs are all the things we register into our compiled-in API type system (Scheme).
var schemeAddFuncs = []func(*runtime.Scheme) error{
	// This is the types only (no defaulters) for all built-in APIs supported by client-go.
	kubernetesscheme.AddToScheme,

	/*
		These are the defaulters for some built-in APIs that we use.

		This code doesn't come with the Kubernetes client library,
		so we have to vendor it from the main server repository.

		Doing this lets us achieve `kubectl apply`-like semantics while
		still using statically-typed Go structs instead of JSON/YAML
		to define our objects.
	*/
	corev1defaults.AddToScheme,
	appsv1defaults.AddToScheme,
	batchv1defaults.AddToScheme,

	// This is our own CRDs.
	planetscaleapis.AddToScheme,
}

// NewScheme returns a Scheme with all the types that the operator needs.
func NewScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	for _, add := range schemeAddFuncs {
		if err := add(scheme); err != nil {
			return nil, err
		}
	}
	return scheme, nil
}
