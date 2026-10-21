# Live resharding demo

One unsharded keyspace, `app`, under a sysbench load it can't keep up with,
resharded live from `-` into `-80` and `80-`. Each tablet's mysqld and vttablet
are capped at 1 CPU, standing in for one database server, so two shards have
twice the capacity of one.

The new shards and the copy are staged before the talk. On stage, the cluster
is already copying into them and keeping up; the live part is the traffic
switch, with the load running.

Needs Docker (OrbStack), kind, kubectl, sysbench, mysql, vtctldclient, and watch.

## Before the talk

```
./demo.sh prep    # online: pull images, build the operator from main
./demo.sh up      # offline from here: cluster, keyspace, data
./demo.sh stage   # new shards, copy, catch up
./demo.sh check   # nothing is waiting on an image pull
```

Rehearse with Wi-Fi off, from `./demo.sh down`.

## On stage

Three panes: `./demo.sh load`, `./demo.sh top`, and one for the steps.

| Step | Command | Point at |
|---|---|---|
| The bottleneck | `load`, `top` | the one primary's mysqld pinned at its 1 CPU (~990m); ~5.7k QPS at ~5.6 ms |
| The plan | `diff cluster-1-unsharded.yaml cluster-2-add-shards.yaml` | three lines of spec added the shards |
| Ready to switch | `status` | copy done, both streams running with 0s lag |
| Move reads | `switch-reads` | instant |
| Move writes | `switch-writes` | ~2 s pause while vtgate buffers, then two primaries share the load (~6.9k QPS) |
| Finish | `complete` | the old shard's Pods terminate within a minute; ~10k QPS at ~3.2 ms |

Between the switch and `complete`, Vitess keeps replicating back into the old
shard so `reverse` stays possible, and that costs CPU. The full gain shows up
once `complete` removes it: a good place to explain why the reverse stream exists.

Measured on a 14-core M-series Mac with OrbStack: 5.7k to 10k QPS (+74%),
average query latency 5.6 to 3.2 ms, one error during the write switch.
After `complete` the laptop itself is the limit (~80% busy), not the shards.

`reverse` undoes the last switch if something goes wrong.

`./demo.sh down` deletes everything afterwards.
