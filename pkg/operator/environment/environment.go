/*
Copyright 2020 PlanetScale Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
Package environment defines global environment variables to call throughout the operator codebase. These variables
have sane defaults and aren't required to be set as flags unless explicitly stated.
*/

package environment

import (
	"time"

	"github.com/spf13/pflag"
)

var (
	reconcileTimeout time.Duration
)

// FlagSet returns the FlagSet for the operator.
func FlagSet() *pflag.FlagSet {
	operatorFlagSet := pflag.NewFlagSet("operator", pflag.ExitOnError)
	operatorFlagSet.DurationVar(&reconcileTimeout, "reconcile_timeout", 10*time.Minute, "Maximum time that any controller will spend trying to reconcile a single object before giving up.")
	return operatorFlagSet
}

// ReconcileTimeout returns the global maximum reconcile timeout for all controllers.
func ReconcileTimeout() time.Duration {
	return reconcileTimeout
}
