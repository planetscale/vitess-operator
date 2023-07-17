# Release Process

This doc describes the process to cut a new release of Vitess Operator.

-------------------

## Prepare for Release

For each major release of the Operator, there is a release branch, for instance: `release-2.7`.
This branch must be created during the RC-1 release.

Before creating proceeding to the actual release, open PR to ensure the following are updated on `HEAD` of the release branch.

### Update GO Version

If Vitess's Go version has been updated since the last release update following files with the corresponding new version used at Vitess.

```console
build/Dockerfile.release
.github/workflows/**.yaml
go.mod
```

-------------------

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

-------------------

## Cut Release

### Release

After the PR from the prepare phase is merged, make sure your local git dir is
up-to-date with the remote's HEAD, and then create a temporary release branch on top of the long-term release branch, for instance:

```
git checkout -b new-release-2.7.5 origin/release-2.7
```

#### Update Vitess Dependency

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

Following this cleanup the `go.sum` file by:
```sh
go mod tidy
```

#### Do-Release

Now, create the tag using the following command, note that you will need to replace the placeholder strings:

```
OLD_VITESS_VERSION="13.0.0" NEW_VITESS_VERSION="14.0.3" NEW_OPERATOR_VERSION="2.7.4" NEXT_OPERATOR_VERSION="2.7.5" ./tools/release/do_release.sh 
```

Here we want to release the version `2.7.4`. It will be tested against Vitess `v14.0.3`. The upgrade downgrade tests will begin with Vitess `v13.0.0`.

[Docker Hub](https://hub.docker.com/repository/docker/planetscale/vitess-operator)
should automatically detect the new tag and begin building a new image.

Follow the instructions prompted by the `do_release.sh` script. You will need to push the tag and push the temporary branch to finally create a Pull Request. The Pull Request should be merged onto the release branch.

> **Note**
> Make sure to Normal Merge the pull request i.e. merge the pull request with merge commit and not a squash merge. This is required because we create the tag
from the pull request, so in order to have the tag on the release branche's history, it has to be a normal merge.

##### Update the test output

The `upgrade_test.sh`, `backup_restore_test.sh` and `vtorc_vtadmin_test.sh` files must be updated with the proper release increment. Change the `verifyVtGateVersion` function calls to use the proper version (current version being released and latest previous version (only used in `upgrade_test.sh`)).

##### CI Failures

> **Note**
> It is likely that the buildkite tests will fail on the release PR initially because of the unavailability of the latest vitess and vitess-operator docker images. This however doesn't block the release. The tests should be restarted after the said images are built and available.

-------------------

### UI Release

Create a [new release](https://github.com/planetscale/vitess-operator/releases/new)
in GitHub UI and make sure to add the release-notes from the docs (if any).

-------------------

### On `main`

> **Note**
> This step is only required for the latest major release and patch releases of the latest major release.

Once you have done the release on the release branch, there are several steps to follow on `main`.

- The `vitess/lite` image tag must be changed in [101_initial_cluster.yaml](..%2Ftest%2Fendtoend%2Foperator%2F101_initial_cluster.yaml). The latest Vitess release tag must be used.
- We must copy the [operator-latest.yaml](..%2Ftest%2Fendtoend%2Foperator%2Foperator-latest.yaml) file we created during the release onto `main`'s [operator.yaml](..%2Ftest%2Fendtoend%2Foperator%2Foperator.yaml) file. Once copied, remove the change that adds `imagePullPolicy: Never` and update the `image: vitess-operator-pr:latest` to use the docker image of latest vitess-operator patch like `image: planetscale/vitess-operator:v2.10.0`.
- The `upgrade_test.sh`, `backup_restore_test.sh` and `vtorc_vtadmin_test.sh` files must be updated with the proper release increment. Change the `verifyVtGateVersion` function calls to use the proper version (new snapshot Vitess version and current version being released (only used in `upgrade_test.sh`)).


