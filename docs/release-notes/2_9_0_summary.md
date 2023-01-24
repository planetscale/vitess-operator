## Major Changes

### EnforceSemiSync Removal

The config `enforceSemiSync` is removed from the `VitessReplicationSpec`. This configuration is no longer requied.
If the users want to configure semi-sync replication, they should set the `durabilityPolicy` config to `semi_sync` in the keyspace specification.
This change of not using `enforceSemiSync` should be done before upgrading to `2.9.0` version of the operator otherwise the configuration would not be accepted.