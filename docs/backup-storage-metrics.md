# Scraping backup storage freshness metrics

The VitessBackupStorage subcontroller inventories backups in object storage
and serves a Prometheus gauge for backup freshness:

```
vitess_operator_backup_last_complete_timestamp_seconds{cluster,keyspace,shard,location}
```

The value is the Unix timestamp of the latest complete backup observed for
that shard and storage location. A backup is complete when a valid MANIFEST
is present in storage, which matches existing `VitessBackup` status. The
latest backup is chosen by `StartTime` (from the backup name). The published
value is `FinishedTime` from the MANIFEST when present, otherwise `StartTime`.

The `shard` label is the Vitess shard name (`VitessShard.spec.name`), such as
`-80` or `-`, not the Kubernetes-safe key range name (`x-80`). That matches
other per-shard operator metrics, such as
`vitess_operator_shard_reconcile_count`.

## Which shards are reported

The operator runs one subcontroller per backup location. Each subcontroller
emits a series only for shards that have at least one tablet pool whose
`backupLocationName` matches its location. A pool without
`backupLocationName` uses the location whose name is empty, so its shard is
reported only by that location's subcontroller. A shard that does not back up
to a location has no series for it, and moving a pool from location `a` to
`b` removes the shard's `a` series on the next inventory of `a`.

Shards that the operator does not back up are not reported by any
subcontroller: a shard with a pool that uses `externalDatastore`, or with a
pool that has no `mysqld`, has no series. This matches the shards for which
the operator configures backups.

## Zero values

The value is `0` until a reported shard has a complete backup at this
location. A new shard, a shard whose backups have never succeeded, or a shard
whose last complete backup was deleted therefore still has a series, and an
RPO alert can fire on it. The gauge is updated only after a successful
inventory; a failed inventory or a failed refresh of `VitessBackup` objects
leaves the previous values in place.

## Missing series

A missing series does not mean the backup age is known. The gauge is kept in
the subcontroller's memory, so there are no series:

- after the subcontroller Pod restarts (for example during an operator
  upgrade or a node drain), until its next successful inventory;
- while the subcontroller Pod is not running or cannot be scraped;
- while every inventory fails, for example because of broken storage
  credentials or more backups than
  `--vitessbackupstorage_subcontroller_max_backups_per_reconcile`.

In those cases an RPO alert on this gauge resolves even though the backup age
is unknown. Pair it with alerts on inventory failures and on the scrape
target, as in the example below.

## How to scrape the subcontroller

The forked subcontroller Pod serves the operator metrics endpoint at
`/metrics` on a container port named `metrics` (`8383`).

`spec.backup.locations[].annotations` are copied onto the subcontroller Pod,
but the same annotations are also applied to vttablet and vtbackup Pods. A
`prometheus.io/port: "8383"` annotation is therefore not a good way to select
the scrape port, because those other Pods would inherit it too.

Instead, select Pods labeled `planetscale.com/component=vbs-subcontroller`
and scrape the `metrics` port.

The operator does not create a Service or `PodMonitor`. Configure discovery
for the Prometheus installation that runs in your cluster, and allow that
traffic in any applicable NetworkPolicy.

### Prometheus Operator `PodMonitor`

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: vitess-backup-storage
  namespace: example
spec:
  namespaceSelector:
    matchNames:
      - example
  selector:
    matchLabels:
      planetscale.com/component: vbs-subcontroller
  podMetricsEndpoints:
    - port: metrics
      path: /metrics
      interval: 30s
```

### Prometheus scrape config

```yaml
scrape_configs:
  - job_name: vitess-backup-storage
    metrics_path: /metrics
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_planetscale_com_component]
        action: keep
        regex: vbs-subcontroller
      - source_labels: [__meta_kubernetes_pod_container_port_name]
        action: keep
        regex: metrics
```

## Example alerts

Because reported shards are exported as `0` until a complete backup exists,
a simple age comparison also catches shards that have never been backed up.
The other two alerts cover the cases where the gauge has no series:

```yaml
groups:
  - name: vitess-backups
    rules:
      - alert: VitessBackupTooOld
        expr: time() - vitess_operator_backup_last_complete_timestamp_seconds > 86400
        for: 15m
        labels:
          severity: critical
        annotations:
          summary: "Vitess backup for {{ $labels.keyspace }}/{{ $labels.shard }} is older than the RPO target"
          description: >-
            The latest complete backup for cluster {{ $labels.cluster }},
            keyspace {{ $labels.keyspace }}, shard {{ $labels.shard }},
            location {{ $labels.location }} is more than 24h old
            (or has never completed, if the metric is 0).
      - alert: VitessBackupInventoryFailing
        expr: increase(vitess_operator_backup_storage_subcontroller_reconcile_count{result="error"}[15m]) > 0
        labels:
          severity: warning
        annotations:
          summary: "Backup inventory for {{ $labels.backup_storage }} is failing"
          description: >-
            The subcontroller for VitessBackupStorage {{ $labels.backup_storage }}
            failed to inventory backups in the last 15m, so backup freshness
            for that location may be stale or missing.
      - alert: VitessBackupStorageScrapeDown
        expr: up{job="vitess-backup-storage"} == 0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Backup storage subcontroller {{ $labels.instance }} cannot be scraped"
```

Adjust the thresholds to your RPO, and the `job` label to match your scrape
configuration: the scrape config above uses `vitess-backup-storage`, while a
`PodMonitor` defaults to `<namespace>/<name>`, here
`example/vitess-backup-storage`. `up == 0` only fires while Prometheus still discovers the
target; it does not catch a subcontroller Pod that does not exist. Manifest
completeness is not proof that a backup can be restored.
