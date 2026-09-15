# swtop

A `btop`/`htop`-style terminal UI for a Docker Swarm cluster.

swtop connects to every node in your swarm over SSH, collects host-level
resource usage (CPU, memory, swap, disk, network, load) and per-container
Docker stats, and shows two kinds of views:

- **Cluster** — the whole swarm treated as one machine: aggregated CPU/mem/
  disk/network gauges, a per-node summary table, and a per-service table
  (containers grouped by their swarm service, cluster-wide).
- **Per-node** — an htop-like view of a single node: per-core CPU bars,
  memory/swap/disk gauges, network throughput, load average, and a table of
  every container running on that node.

## Why SSH?

swtop talks to each node directly over SSH rather than through the Docker
Swarm manager API. That means:

- It works the same for managers and workers, with no special privileges
  needed on the manager.
- It can read real host-level OS metrics (`/proc/stat`, `/proc/meminfo`,
  `/proc/net/dev`, `df`) that the Docker API doesn't expose for remote
  nodes — the same numbers `htop` would show you if you were logged into
  that machine.
- Docker container/service stats are read from each node's own local
  Docker engine (`docker stats` / `docker ps`), so no manager access is
  required at all — swarm service membership is inferred from the
  `com.docker.swarm.service.name` label Docker attaches to every task
  container.

The tradeoff is that swtop needs SSH access (key or agent-based) to every
node you want to monitor.

## Requirements

On your machine running swtop:
- Go 1.24+ (only if building from source)
- An SSH key or `ssh-agent` that can authenticate to every node
- Each node already present in `~/.ssh/known_hosts` (connect once manually
  with `ssh` if not — swtop verifies host keys and will not silently trust
  an unknown host)

On each swarm node:
- A standard SSH server
- `/proc` (any modern Linux) and `df` (coreutils)
- `docker` CLI on the `PATH` for the SSH user, able to talk to the local
  Docker engine (i.e. that user is in the `docker` group or is root)

## Build

```sh
go build -o swtop ./cmd/swtop
```

## Configure

Copy the example config and point it at your nodes:

```sh
cp config.example.yaml swtop.yaml
$EDITOR swtop.yaml
```

```yaml
poll_interval: 2s

ssh:
  user: root
  identity_file: ~/.ssh/id_rsa
  port: 22
  timeout: 5s

nodes:
  - name: manager-1
    address: 10.0.0.10
    role: manager
  - name: worker-1
    address: 10.0.0.11
    role: worker
```

Per-node `user`, `port`, and `identity_file` override the `ssh` defaults.

swtop looks for `~/.config/swtop/config.yaml` by default, or a `swtop.yaml`
in the current directory; pass `-config /path/to/file.yaml` to use another
location.

## Run

```sh
./swtop -config swtop.yaml
```

### Keys

| Key                 | Action                                  |
|---------------------|------------------------------------------|
| `tab` / `→` / `l`    | Next view (Cluster → node 1 → node 2 …) |
| `shift+tab` / `←` / `h` | Previous view                        |
| `0`-`9`              | Jump directly to Cluster (`0`) or node N |
| `↑`/`↓`, `pgup`/`pgdn` | Scroll the tables when they don't fit the terminal |
| `s`                  | Toggle table sort between CPU% and memory |
| `q` / `ctrl+c`       | Quit                                     |

## How it works

Each node runs its own polling goroutine on a fixed interval
(`poll_interval`). Every poll is a single SSH command execution that reads
`/proc/stat`, `/proc/meminfo`, `/proc/loadavg`, `/proc/uptime`,
`/proc/net/dev`, `df -kP /`, `docker stats --no-stream`, and `docker ps` in
one round trip. CPU% and network throughput are derived from the delta
between consecutive polls (the same technique `top`/`htop` use), so the
first sample after startup briefly reads as 0%.

A node that's unreachable is shown as offline in the Cluster view and
excluded from the aggregated totals until it responds again; swtop keeps
retrying it every poll interval.

## Project layout

```
cmd/swtop/          entrypoint: config loading, wiring, bubbletea program
internal/config/    YAML config loading and validation
internal/sshx/      minimal reconnecting SSH command runner
internal/probe/     the remote collection script + output parsing
internal/model/     shared data types and cluster-wide aggregation
internal/collector/ per-node polling goroutines -> cluster snapshots
internal/tui/       bubbletea UI (cluster view, node view, gauges, tables)
```
