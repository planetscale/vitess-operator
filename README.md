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

### Build Docker image

From this directory, run:

```
make build IMAGE_NAME=your.registry/vitess/operator
```

## Contributing

If you would like to contribute to this project, please refer to the
[contributing readme](CONTRIBUTING.md)

