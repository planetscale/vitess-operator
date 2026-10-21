#!/usr/bin/env bash
# Live resharding demo driver. Run `./demo.sh help` for the steps.
#
# `prep` is the only step that needs the internet. Everything after it runs
# offline from the local Docker image cache.
set -euo pipefail
cd "$(dirname "$0")"

# The operator is built from the checkout this script lives in.
OPERATOR_SRC="${OPERATOR_SRC:-$(git rev-parse --show-toplevel)}"
KIND_CLUSTER=vitess-demo
NS=example
KS=app
WORKFLOW="split"
TABLES="${TABLES:-4}"
TABLE_SIZE="${TABLE_SIZE:-250000}"
THREADS="${THREADS:-32}"
# Homebrew's sysbench uses MariaDB Connector/C 3.4+, which refuses plaintext
# connections (vtgate here has no TLS) unless this is set. --mysql-ssl=off has no effect.
export MARIADB_TLS_DISABLE_PEER_VERIFICATION=1

OPERATOR_IMAGE=vitess-operator-demo:local
METRICS_SERVER_VERSION=v0.8.0
IMAGES=(
  kindest/node:v1.34.0
  vitess/lite:v24.0.5-mysql80
  quay.io/coreos/etcd:v3.5.17
  prom/mysqld-exporter:v0.18.0
  "registry.k8s.io/metrics-server/metrics-server:${METRICS_SERVER_VERSION}"
)

vtctld() { command vtctldclient --server=127.0.0.1:15999 "$@"; }
sql() { command mysql --host=127.0.0.1 --port=15306 --user=user "$@"; }
say() { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }
# Print a command, then run it, so the audience sees exactly what's typed.
show() { local c="$*"; printf '\033[1;33m$ %s\033[0m\n' "${c/#vtctld /vtctldclient }"; "$@"; }

sysbench_args() {
  echo --db-driver=mysql --mysql-host=127.0.0.1 --mysql-port=15306 --mysql-user=user \
    --mysql-db="$KS" --tables="$TABLES" --table-size="$TABLE_SIZE" \
    --auto_inc=off --db-ps-mode=disable
}

# wait_tablets SHARD... waits until each shard has a primary and all of the
# keyspace's tablet Pods are Ready.
wait_tablets() {
  local s
  for s in "$@"; do
    printf 'Waiting for a primary in %s/%s' "$KS" "$s"
    until vtctld GetTablets --keyspace "$KS" --shard "$s" 2>/dev/null | grep -q ' primary '; do
      printf .; sleep 3
    done
    echo
  done
  kubectl wait pods -n "$NS" -l planetscale.com/component=vttablet --for=condition=Ready --timeout=10m >/dev/null
}

cmd_prep() {
  say "Pulling images (needs internet)"
  local i
  for i in "${IMAGES[@]}"; do docker pull "$i"; done
  say "Building the operator from ${OPERATOR_SRC} ($(git -C "$OPERATOR_SRC" log -1 --format='%h %s'))"
  docker build --file "$OPERATOR_SRC/build/Dockerfile.release" --tag "$OPERATOR_IMAGE" "$OPERATOR_SRC"
  perl -pe "s|image: planetscale/vitess-operator:.*|image: ${OPERATOR_IMAGE}\n          imagePullPolicy: IfNotPresent|" \
    "$OPERATOR_SRC/test/endtoend/operator/operator.yaml" > operator.yaml
  say "Downloading metrics-server ${METRICS_SERVER_VERSION}"
  curl -fsSL "https://github.com/kubernetes-sigs/metrics-server/releases/download/${METRICS_SERVER_VERSION}/components.yaml" |
    perl -pe "s|- --metric-resolution=15s|- --metric-resolution=15s\n        - --kubelet-insecure-tls|" > metrics-server.yaml
  grep -q kubelet-insecure-tls metrics-server.yaml
  say "Done. Everything from here on works offline."
}

cmd_up() {
  say "Creating the kind cluster"
  kind create cluster --name "$KIND_CLUSTER" --config kind.yaml --wait 2m
  say "Loading images into the cluster (no registry access needed)"
  local i
  for i in "${IMAGES[@]:1}" "$OPERATOR_IMAGE"; do kind load docker-image "$i" --name "$KIND_CLUSTER"; done
  kubectl apply -f metrics-server.yaml
  # The e2e operator manifest watches the "example" namespace, created here.
  kubectl apply -f base.yaml
  kubectl apply --server-side -f operator.yaml
  kubectl rollout status deployment/vitess-operator --timeout=3m
  kubectl apply -f cluster-1-unsharded.yaml
  until kubectl get svc -n "$NS" -l planetscale.com/component=vtctld -o name 2>/dev/null | grep -q .; do sleep 2; done
  wait_tablets -
  # vtgate learns about the primary from its health check, a little after the topo.
  until sql --database="$KS" --execute "show tables" >/dev/null 2>&1; do sleep 2; done
  say "Applying the VSchema: rows will be distributed by hash(id)"
  vtctld ApplyVSchema --vschema-file <(vschema) "$KS" >/dev/null
  cmd_data
  say "Ready. Next: ./demo.sh stage"
}

vschema() {
  local t sep=""
  printf '{"sharded": true, "vindexes": {"hash": {"type": "hash"}}, "tables": {'
  for ((t = 1; t <= TABLES; t++)); do
    printf '%s"sbtest%d": {"column_vindexes": [{"column": "id", "name": "hash"}]}' "$sep" "$t"
    sep=", "
  done
  printf '}}\n'
}

cmd_data() {
  say "Loading ${TABLES} tables x ${TABLE_SIZE} rows"
  # shellcheck disable=SC2046 # word splitting is intended
  sysbench oltp_read_write $(sysbench_args) --threads="$TABLES" prepare 2>&1 | cut -c1-200
  # sysbench exits 0 even when prepare fails, so check the result.
  local rows
  rows="$(sql --skip-column-names --execute "select count(*) from ${KS}.sbtest${TABLES}")"
  [[ "$rows" == "$TABLE_SIZE" ]] || { echo "Expected ${TABLE_SIZE} rows in sbtest${TABLES}, found ${rows}" >&2; return 1; }
}

# The workload: 10 point selects and 1 update per event, all by primary key,
# so every query goes to exactly one shard. Errors are ignored so the load keeps
# running through the traffic switch.
#
# Each thread has one query in flight at a time, so the average query latency
# is threads / QPS. (sysbench 1.0.20's per-interval p95 prints 0.00 here.)
cmd_load() {
  local args
  # shellcheck disable=SC2207
  args=($(sysbench_args) --range_selects=off --delete_inserts=0 --non_index_updates=0 --skip_trx=on --threads="$THREADS" --time=0 --report-interval=1 --mysql-ignore-errors=all)
  printf '\033[1;33m$ sysbench oltp_read_write %s run\033[0m\n' "${args[*]}"
  sysbench oltp_read_write "${args[@]}" run 2>&1 |
    awk -v threads="$THREADS" '
      /^\[/ { qps = $9; printf "%6s   QPS %7.0f   avg query latency %6.2f ms   errors/s %3.0f\n", $2, qps, (qps > 0 ? threads * 1000 / qps : 0), $(NF-2); fflush() }
      /FATAL|ERROR/ { print; fflush() }'
}

# Live CPU use of every tablet, labeled by shard and role.
cmd_top() {
  # $0 may be relative to where we started, before the cd above.
  exec watch -t -n 2 "./$(basename "$0")" top-once
}

cmd_top-once() {
  printf '%-6s %-8s %-42s %8s %9s\n' SHARD ROLE POD MYSQLD VTTABLET
  # Join "alias shard role" from the topo with per-container CPU by tablet UID.
  awk '
    NR == FNR { uid = $1; sub(/^zone1-/, "", uid); shard[uid] = $3; role[uid] = $4; next }
    { uid = $1; sub(/^.*-vttablet-zone1-/, "", uid); sub(/-.*$/, "", uid); pods[$1] = uid
      if ($2 == "mysqld") m[$1] = $3; else if ($2 == "vttablet") v[$1] = $3 }
    END { for (p in pods) printf "%-6s %-8s %-42s %8s %9s\n", shard[pods[p]], role[pods[p]], p, m[p], v[p] }' \
    <(vtctld GetTablets --keyspace "$KS" 2>/dev/null) \
    <(kubectl top pods -n "$NS" -l planetscale.com/component=vttablet --containers --no-headers 2>/dev/null) |
    sort
}

cmd_shards() {
  say "Adding shards -80 and 80- next to the existing shard -"
  show diff -u cluster-1-unsharded.yaml cluster-2-add-shards.yaml || true
  show kubectl apply -f cluster-2-add-shards.yaml
  wait_tablets -80 80-
  say "New shards are up, empty, and not serving yet"
}

# Streaming from the source primary rather than its replica keeps the copy from
# falling behind: the replica's 1 CPU is already busy applying the load's writes.
cmd_reshard() {
  show vtctld Reshard --workflow "$WORKFLOW" --target-keyspace "$KS" create --source-shards '-' --target-shards '-80,80-' --tablet-types primary
}

# Before the talk: new shards, copy, and catch up, ready for switchtraffic.
cmd_stage() {
  cmd_shards
  cmd_reshard
  say "Copying (no load running, so this goes fastest)"
  # Done when every stream is past the copy and reports a lag ("Running.
  # VStream Lag: 0s"), not "Copying" or "VStream has not started".
  local out streams
  while :; do
    out="$(vtctld Reshard --workflow "$WORKFLOW" --target-keyspace "$KS" status)"
    streams="$(grep -c '^id=' <<<"$out" || true)"
    [[ "$streams" -gt 0 && "$streams" == "$(grep -c 'Status: Running. VStream Lag: [0-9]*s' <<<"$out" || true)" ]] && break
    printf .; sleep 5
  done
  echo
  cmd_status
  say "Staged. Start the load and switch traffic on stage."
}

cmd_status() {
  show vtctld Reshard --workflow "$WORKFLOW" --target-keyspace "$KS" status
}

cmd_vdiff() {
  show vtctld VDiff --workflow "$WORKFLOW" --target-keyspace "$KS" create --wait
}

cmd_switch-reads() {
  show vtctld Reshard --workflow "$WORKFLOW" --target-keyspace "$KS" switchtraffic --tablet-types replica,rdonly
}

cmd_switch-writes() {
  show vtctld Reshard --workflow "$WORKFLOW" --target-keyspace "$KS" switchtraffic --tablet-types primary
}

cmd_reverse() {
  show vtctld Reshard --workflow "$WORKFLOW" --target-keyspace "$KS" reversetraffic
}

cmd_complete() {
  show vtctld Reshard --workflow "$WORKFLOW" --target-keyspace "$KS" complete
  say "Removing the old shard from the spec"
  show diff -u cluster-2-add-shards.yaml cluster-3-sharded.yaml || true
  show kubectl apply -f cluster-3-sharded.yaml
}

cmd_down() {
  kind delete cluster --name "$KIND_CLUSTER"
}

# Fails if anything in the cluster is waiting on an image pull.
cmd_check() {
  if kubectl get pods -A --no-headers | grep -E 'ErrImagePull|ImagePullBackOff'; then
    echo "Some images were not preloaded" >&2
    return 1
  fi
  echo "No image pulls pending."
}

cmd_help() {
  cat <<EOF
Before the talk (online):   prep
Before the talk (offline):  up, stage, check
On stage:
  load           sysbench through vtgate (leave running in a pane)
  top            per-tablet CPU by shard and role (leave running in a pane)
  status         the copy is done and caught up
  switch-reads   route replica reads to the new shards
  switch-writes  route primary traffic to the new shards
  reverse        undo the last switch
  complete       finish the workflow and remove the old shard
Pieces of stage, if you'd rather show them:  shards, reshard, vdiff
Afterwards:  down
EOF
}

cmd="${1:-help}"
declare -F "cmd_${cmd}" >/dev/null || { cmd_help; exit 1; }
"cmd_${cmd}"
