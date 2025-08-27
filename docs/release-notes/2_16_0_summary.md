## Major Changes

### Table of Contents

- **[Upgrade Path](#upgrade-path)**
  - **[MySQL](#mysql-upgrade-path)**

### <a id="upgrade-path"/>Upgrade Path</a>

#### <a id="mysql-upgrade-path"/>MySQL</a>

With the latest version of Vitess (`v23.0.0`) the default MySQL version changed from `8.0.40` to `8.4.6`.

In order for you to correctly upgrade, there is a certain path to follow:

1. Add `innodb_fast_shutdown=0` to your extra cnf in your YAML file.
2. Apply this file.
3. Wait for all the pods to be healthy.
4. Then change your YAML file to use the new Docker Image (`mysql:8.4.6`).
5. Remove `innodb_fast_shutdown=0` from your extra cnf in your YAML file.
6. Apply this file.
