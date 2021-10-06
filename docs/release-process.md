# Release Process

This doc describes the process to cut a new release of Vitess Operator.

## Prepare for Release

Before creating a release tag, send a PR to ensure the following are updated on
HEAD of the main branch, if necessary.

### Update GO Version

If Vitess's Go version has been updated since the last release update following files with the corresponding new version used at Vitess. 
```console
build/Dockerfile.release
.github/workflows/integration-test.yaml
.github/workflows/make-generate-and-diff.yaml
.github/workflows/unit-test.yaml
go.mod
```
### Update Vitess Dependency

Each Vitess Operator minor version (`vX.Y.*`) is intended to correspond to a
particular Vitess major version (`vX.*.*`).
See the [compatibility table](https://github.com/planetscale/vitess-operator#compatibility)
for examples.

If you're cutting a new minor version, you should start by updating the version
of Vitess code that we build against:

```console
planetscale.dev/vitess-operator$ go get vitess.io/vitess@<git-commit-hash>
```

Note that the Go module version must be a git commit sha and not a tag like
`v9.0.0` because the Vitess repository doesn't follow all the `go mod` rules for
semantic versioning. You can look up the commit hash for a tag from the
[Vitess tags](https://github.com/vitessio/vitess/tags) page.

For example, the `v9.0.0` tag corresponds to commit `daa60859822ff85ce18e2d10c61a27b7797ec6b8`
so this command would update Vitess Operator to build against Vitess v9.0.0:

```sh
go get vitess.io/vitess@daa60859822ff85ce18e2d10c61a27b7797ec6b8
```

### Update Default Vitess Image

In addition to being built against Vitess code, Vitess Operator also deploys
Vitess itself with Docker images. To update the default Vitess version that the
operator deploys when the user doesn't specify, set this constant to the
desired `vitess/lite` image URL:

https://github.com/planetscale/vitess-operator/blob/4f37e79173c41f9a6a4ab50e45af7c5e959dbb0b/pkg/apis/planetscale/v2/defaults.go#L100

Note that this image tag is pushed by the Vitess project, and it doesn't necessarily
have to exist yet in order to complete the Vitess Operator release cut process.
This allows flexibility in the coordination of Vitess and Vitess Operator releases,
but it means the resulting operator will fail to actually deploy Vitess with default
settings until the new `vitess/lite` image tag is pushed.

### Update Compatibility Table

Add a new entry for the planned minor version to the [compatibility table](https://github.com/planetscale/vitess-operator/blob/main/README.md#compatibility)
in the README file.

The recommended Kubernetes versions depend on the version of the Kubernetes
client libs that Vitess Operator uses:

https://github.com/planetscale/vitess-operator/blob/111ac173e1c473853e66e270486ba8a9a47ecc54/go.mod#L35

The recommended server versions are those that match the minor version for the
client libraries, as well as server versions that are +/- one minor version
relative to the client libraries.

Note that we don't necessarily update to newer Kubernetes libraries at the same
time that we update to build against a newer Vitess release.
The Kubernetes library version is determined by the version of [Operator SDK](https://github.com/operator-framework/operator-sdk)
that's in use.

## Cut Release

After the PR from the prepare phase is merged, make sure your local git dir is
up-to-date with HEAD and then create and push a new tag. For example:

```sh
git tag v2.3.0
git push origin v2.3.0
```

[Docker Hub](https://hub.docker.com/repository/docker/planetscale/vitess-operator)
hould automatically detect the new tag and begin building a new image.

Create a [new release](https://github.com/planetscale/vitess-operator/releases/new)
in GitHub to describe the updates users should expect.

## After Release

After a release cut, the repo's main branch serves as the home for new development
towards the next release. To indicate this, update the version information that
gets baked into the binary:

https://github.com/planetscale/vitess-operator/blob/111ac173e1c473853e66e270486ba8a9a47ecc54/version/version.go#L20

This version should be one minor version above the release you just cut.
```console
2.5.0 --> 2.6.0
```
