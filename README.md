# Vitess Operator

## Docs

- [Overview](https://docs.planetscale.com/vitess-operator/overview)
- [Getting Started on AWS](https://docs.planetscale.com/vitess-operator/aws-quickstart)
- [Getting Started on GCP](https://docs.planetscale.com/vitess-operator/gcp-quickstart)
- [VitessCluster CRD API Reference](https://docs.planetscale.com/vitess-operator/api)

## Compatibility

Vitess Operator depends on Vitess libraries and Kubernetes libraries that
each support a limited range of Vitess and Kubernetes versions, respectively.
These limitations mean each Vitess Operator version is only guaranteed to be
compatible with certain Vitess and Kubernetes versions, as shown in this table:

Vitess Operator Version | Recommended Vitess Versions | Recommended Kubernetes Versions
--- | --- | ---
`v2.0.*` | `v6.0.*` | `v1.13.*`, `v1.14.*`, or `v1.15.*`
`v2.1.*` | `v7.0.*` | `v1.15.*`, `v1.16.*`, or `v1.17.*`
`v2.2.*` | `v8.0.*` | `v1.15.*`, `v1.16.*`, or `v1.17.*`
`v2.3.*` | `v9.0.*` | `v1.15.*`, `v1.16.*`, or `v1.17.*`
`latest` | `latest` | `v1.15.*`, `v1.16.*`, or `v1.17.*`

If for some reason you must attempt to use versions outside the recommend
window, we still welcome bug reports since a workaround might be possible.
However, in some cases we may be unable to overcome the underlying limitations
in our dependencies.

### Update Schedule

Recommended versions for HEAD of Vitess Operator can change over time.
However, patch releases (e.g. `v2.0.*`) will retain the same compatibility windows
as the original release in that minor series (e.g. `v2.0.0`).

We plan to update HEAD of Vitess Operator to work with each new Vitess version
soon after it's released. Once we're confident in the compatibility of the new
pairing, we'll cut a new release of Vitess Operator while incrementing the minor
version (the `Y` in `vX.Y.Z`).

Our goal for Kubernetes is for the latest Vitess Operator release to be
compatible with the latest Kubernetes version that's Generally Available on all
of GKE, EKS, and AKS. If we need to update our Kubernetes library dependencies
to keep the target Kubernetes version in our compatibility window, we'll update
HEAD of Vitess Operator and then cut a new minor release once we're confident in
the new pairing.

## Build

This secton describes how to build your own Vitess Operator binaries and images.
See the Getting Started guides above if you just want to deploy Vitess Operator
from pre-built images.

### Build Docker image

From this directory, run:

```
make build IMAGE_NAME=your.registry/vitess/operator
```

## Contributing

If you would like to contribute to this project, please refer to the
[contributing readme](CONTRIBUTING.md)

