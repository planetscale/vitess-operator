## Major Changes

### EnforceSemiSync Removal

The config `enforceSemiSync` is removed from the `VitessReplicationSpec`. This configuration is no longer requied.
If the users want to configure semi-sync replication, they should set the `durabilityPolicy` config to `semi_sync` in the keyspace specification.
This change of not using `enforceSemiSync` should be done before upgrading to `2.9.0` version of the operator otherwise the configuration would not be accepted.

### VTOrc becomes mandatory

VTOrc is now a **required** component of Vitess starting from v16. So, the vitess-operator will always run
a VTOrc instance for a keyspace, even if its configuration is unspecified.

### MySQL Version Upgrade

The Vitess-Operator will now support minor MySQL version upgrade. For instance from MySQL `8.0.23` to `8.0.31`.
The global variable `innodb_fast_shutdown` will be set to `0` before shutting down the `mysqld` container.