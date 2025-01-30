## Major Changes

### etcd Upgrade Path

We have changed the default etcd version to `3.5.17`.

You can upgrade by changing your YAML file to use the new Docker Image (`quay.io/coreos/etcd:v3.5.17`).

### MySQL Upgrade Path

With the latest version of Vitess (`v22.0.0`) the default MySQL version changed from `8.0.30` to `8.0.40`.

In order for you to correctly upgrade, there is a certain path to follow:

1. Add `innodb_fast_shutdown=0` to your extra cnf in your YAML file.
2. Apply this file.
3. Wait for all the pods to be healthy.
4. Then change your YAML file to use the new Docker Image (`mysql:8.0.40`).
5. Remove `innodb_fast_shutdown=0` from your extra cnf in your YAML file.
6. Apply this file.
