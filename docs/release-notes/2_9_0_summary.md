## Major Changes

### EnforceSemiSync Removal

The config `enforceSemiSync` is removed from the `VitessReplicationSpec`. This configuration is no longer requied.
If the users want to configure semi-sync replication, they should set the `durabilityPolicy` config to `semi_sync` in the keyspace specification.
This change of not using `enforceSemiSync` should be done before upgrading to `2.9.0` version of the operator otherwise the configuration would not be accepted.

### VTOrc becomes mandatory

VTOrc is now a **required** component of Vitess starting from v16. So, the vitess-operator will always run
a VTOrc instance for a keyspace, even if its configuration is unspecified.

### MySQL Upgrade Path

With the latest version of Vitess (`v16.0.0`) the default MySQL version changed from `5.7` to `8.0.30`.
Meaning that the `vitess/lite:v15.0.2` and `vitess/lite:v16.0.0` are running different MySQL major version.
If you want to remain on MySQL 5.7, we invite you to use `vitess/lite:v16.0.0-mysql57`.

Otherwise, if you were already running MySQL 5.7, with for instance `vitess/lite:v15.0.2-mysql80`, note that here the patch version of MySQL will also change between `v15` and `v16`.
In `v16.0.0` we are bumping the patch version of MySQL 80 from `8.0.23` to `8.0.30`.
In order for you to correctly upgrade, there is a certain path to follow:

1. Add `innodb_fast_shutdown=0` to your extra cnf in your YAML file.
2. Apply this file.
3. Wait for all the pods to be healthy.
4. Then change your YAML file to use the new Docker Images (`vitess/lite:v16.0.0`, defaults to mysql80).
5. Wait for all the pods to be healthy.
6. Remove `innodb_fast_shutdown=0` from your extra cnf in your YAML file.
7. Apply this file.
