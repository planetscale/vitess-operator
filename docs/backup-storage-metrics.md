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

## Zero values

The subcontroller emits a series for every shard it inventories. The value is
`0` until that shard has a complete backup at this location. A successful
inventory of a new shard, a shard whose backups have never succeeded, or a
shard whose last complete backup was deleted therefore still has a series, and
an RPO alert can fire on it. The gauge is updated only after a successful
inventory; a failed inventory or a failed refresh of `VitessBackup` objects
leaves the previous values in place.

A missing series means the shard is not in the current inventory (for example,
it was deleted), not that its backup age is unknown.

## How to scrape the subcontroller

The forked subcontroller Pod serves the operator metrics endpoint on port
`8383` at `/metrics`. The subcontroller container declares no ports, so
Prometheus cannot discover that port from the Pod spec.

`spec.backup.locations[].annotations` are copied onto the subcontroller Pod,
but the same annotations are also applied to vttablet and vtbackup Pods. A
`prometheus.io/port: "8383"` annotation is therefore not a good way to select
the scrape port, because those other Pods would inherit it too.

Instead, select Pods labeled `planetscale.com/component=vbs-subcontroller`
and scrape port `8383` directly.

The operator does not create a Service or `PodMonitor`. Configure discovery
for the Prometheus installation that runs in your cluster, and allow that
traffic in any applicable NetworkPolicy.

### Prometheus Operator `PodMonitor`

Use `targetPort` (a number) because the container has no named port:

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
    - targetPort: 8383
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
      - source_labels: [__meta_kubernetes_pod_ip]
        action: replace
        target_label: __address__
        replacement: $1:8383
```

## Example RPO alert

Because known shards are exported as `0` until a complete backup exists, a
simple age comparison also catches shards that have never been backed up:

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
```

Adjust the threshold to your RPO. Manifest completeness is not proof that a
backup can be restored.
