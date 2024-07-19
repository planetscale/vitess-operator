# Vitess Operator

## Docs

- [Overview](docs/)
- [Getting Started on AWS](docs/aws-quickstart.md)
- [Getting Started on GCP](docs/gcp-quickstart.md)
- [VitessCluster CRD API Reference](docs/api.md)

## Compatibility

Vitess Operator depends on Vitess libraries and Kubernetes libraries that
each support a limited range of Vitess and Kubernetes versions, respectively.
These limitations mean each Vitess Operator version is only guaranteed to be
compatible with certain Vitess and Kubernetes versions, as shown in this table:

| Vitess Operator Version | Recommended Vitess Versions | Recommended Kubernetes Versions               |
|-------------------------|-----------------------------|-----------------------------------------------|
| `v2.11.*`               | `v18.0.*`                   | `v1.22.*`, `v1.23.*`, `v1.24.*`, or `v1.25.*` |
| `v2.12.*`               | `v19.0.*`                   | `v1.25.*`, `v1.26.*`, `v1.27.*`, or `v1.28.*` |
| `v2.13.*`               | `v20.0.*`                   | `v1.25.*`, `v1.26.*`, `v1.27.*`, or `v1.28.*` |
| `latest`                | `latest`                    | `v1.25.*`, `v1.26.*`, `v1.27.*`, or `v1.28.*` |

If for some reason you must attempt to use versions outside the recommend
window, we still welcome bug reports since a workaround might be possible.
However, in some cases we may be unable to overcome the underlying limitations
in our dependencies.

### Release Cycle

For each major release of Vitess there will be a minor release of the vitess-operator.
Each minor release of the vitess-operator follows the same lifecycle as Vitess' releases:
1-year-long lifespan leading to an EOL the same day as the corresponding Vitess major release.

We might release new patch release on a need basis or when doing a patch release in Vitess.
Doing a patch release in Vitess does not necessarily means that there will be a corresponding
patch release in the vitess-operator, the release lead will take the decision based on what changed
in the operator since the last patch or major release.

### Supported Kubernetes Versions

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

