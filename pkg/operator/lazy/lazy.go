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
Package lazy can be used to define placeholders for values that will be computed later.

Each struct in this package maintains a list of provider functions.
These providers are usually added to the list at package initialization
time in init() functions.

In this way, you can define at initialization time a set of sub-computations
that must be done to compute a combined value, even though the information
needed to actually carry out that computation is not yet available. When that
information (represented by the Spec interface) becomes available, you can pass
it to Get() to resolve the lazy value to a concrete value.

This allows complex structures like Pod or Container specs to be expressed in Go
code as simple struct literals, so they resemble the declarative form in which
Kubernetes APIs were designed to be viewed.

The use of init() functions means new providers can be added by adding new
files in a package, without the need to update a central list of which provider
functions to call for a given lazy value. This makes it easy to group providers
into separate functions and files based on the high-level feature they implement,
instead of forcing a grouping based on which computations they affect.

For example, you can add a feature that requires injecting various Volumes,
VolumeMounts, EnvVars, and Annotations by just adding one file that has all
the logic for that feature in one place, instead of having to go around and
update the Volume code, the VolumeMount code, the EnvVar code, and the
Annotation code in separate places.
*/
package lazy

// Spec is a placeholder for the object that contains the info necessary to
// resolve a lazy value. It's just an empty interface, but giving the type a name
// helps to indicate its purpose.
type Spec interface{}
