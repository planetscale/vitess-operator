## Major Changes

### Automated and Scheduled Backups

As part of `v2.13.0` we are adding a new feature to the `vitess-operator`: automated and scheduled backups. The PR
implementing this change is available here: [#553](https://github.com/planetscale/vitess-operator/pull/553).

This feature is for now experimental as we await feedback from the community on its usage. There are a few things to
take into account when using this feature:

- If you are using the `xtrabackup` engine, your vttablet pods will need more memory, think about provisioning more memory for it.
- If you are using the `builtin` engine, you will lose a replica during the backup, think about adding a new tablet.

If you are usually using specific flags when taking backups with `vtctldclient` you can set those flags on the `extraFlags`
field of the backup `strategy` (`VitessBackupScheduleStrategy`).