# Vitess Operator

## Docs

- [Overview](https://docs.planetscale.com/vitess-operator/overview)
- [Getting Started on AWS](https://docs.planetscale.com/vitess-operator/aws-quickstart)
- [Getting Started on GCP](https://docs.planetscale.com/vitess-operator/gcp-quickstart)
- [VitessCluster CRD API Reference](https://docs.planetscale.com/vitess-operator/api)

## Build

This secton describes how to build your own Vitess Operator binaries and images.
See the Getting Started guides above if you just want to deploy Vitess Operator
from pre-built images.

### Prerequisites

Install [Operator SDK](https://github.com/operator-framework/operator-sdk/blob/master/doc/user/install-operator-sdk.md)
version 0.10.0 and rename it to `operator-sdk-v0.10.0` in your path.
We include the version in the file name to ensure that builds will fail if the
installed version is incorrect.

### Build Docker image

From this directory, run:

```
make build IMAGE_NAME=your.registry/vitess/operator
```

## Contributing

If you would like to contribute to this project, please refer to the
[contributing readme](CONTRIBUTING.md)

