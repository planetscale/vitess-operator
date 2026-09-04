# Scraping scheduled vtbackup metrics

Scheduled backups that use `backupMethod: vtbackup` run a short-lived
`vtbackup` Pod. The process serves Prometheus metrics at `/metrics` through
its HTTP server.

## Configure the HTTP port

The operator configures vtbackup to use the standard Vitess web port, `15000`,
by default. Set the existing `vttablet.vtbackupExtraFlags` field on the tablet
pool template to choose a different port:

```yaml
spec:
  keyspaces:
    - name: commerce
      partitionings:
        - equal:
            parts: 1
            shardTemplate:
              tabletPools:
                - cell: zone1
                  type: replica
                  vttablet:
                    vtbackupExtraFlags:
                      port: "18080"
```

The generated vtbackup container declares the configured value as the named
`web` TCP port. The setting is copied to scheduled vtbackup Jobs from the
tablet pool template used to build the backup Pod. If `port` is omitted, the
operator uses `15000`. Set `port: "0"` explicitly to retain vtbackup's
ephemeral behavior and omit the container-port declaration.

Avoid assigning a port already used by another container in the same Pod,
because containers share a network namespace.

## Prometheus annotations

`VitessBackupSchedule.spec.annotations` is copied to generated Jobs and Pod
templates. For annotation-based Prometheus discovery, configure the schedule
as follows:

```yaml
spec:
  backup:
    schedules:
      - name: daily
        backupMethod: vtbackup
        annotations:
          prometheus.io/scrape: "true"
          prometheus.io/port: "15000"
          prometheus.io/path: "/metrics"
```

The annotation port must match the `port` value passed to vtbackup.

## Prometheus Operator `PodMonitor`

The controller labels scheduled vtbackup Jobs and their Pod templates with
`planetscale.com/component: vtbackup` and
`planetscale.com/backup-method: vtbackup`. A `PodMonitor` can select those
Pods and target the named port:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: vitess-vtbackup
  namespace: example
spec:
  namespaceSelector:
    matchNames:
      - example
  selector:
    matchLabels:
      planetscale.com/component: vtbackup
      planetscale.com/backup-method: vtbackup
  podMetricsEndpoints:
    - port: web
      path: /metrics
      interval: 30s
```

Because backup Pods normally exit after the backup completes, a short-lived
Pod may finish before a scrape occurs. Set `keep-alive-timeout` through the
same existing vtbackup flag map when final metrics need to remain available:

```yaml
spec:
  keyspaces:
    - name: commerce
      partitionings:
        - equal:
            parts: 1
            shardTemplate:
              tabletPools:
                - cell: zone1
                  type: replica
                  vttablet:
                    vtbackupExtraFlags:
                      port: "15000"
                      keep-alive-timeout: "60s"
  backup:
    schedules:
      - name: daily
        backupMethod: vtbackup
        jobTimeoutMinute: 15
```

Choose a keep-alive duration of at least twice the Prometheus scrape interval
when possible. The duration counts toward `jobTimeoutMinute`, and
`keep-alive-timeout` only runs after a successful backup; failed backups exit
without waiting for the keep-alive period.

Do not set `stats_backend: prometheus` for vtbackup. That setting selects a
push-style backend and can cause vtbackup to wait for a backend that is not
registered. The HTTP endpoint and `/metrics` path work without it.
