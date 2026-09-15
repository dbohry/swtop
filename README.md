# swtop

[![Latest Release](https://img.shields.io/github/v/release/dbohry/swtop)](https://github.com/dbohry/swtop/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/dbohry/swtop)](go.mod)

A `btop`/`htop`-style terminal UI for a Docker Swarm cluster. Connects to every node over SSH and shows a consolidated cluster view plus a per-node view (CPU, memory, disk, network, and containers).

Plain servers with no Docker at all are welcome too — list them under `servers` and swtop just shows host resources for them, no containers panel, no `docker` calls made.

<img width="1258" height="657" alt="Screenshot 2026-09-15 at 15 46 10" src="https://github.com/user-attachments/assets/75fb82d9-9e58-4304-beb2-aadbf708650e" />
<img width="1261" height="656" alt="Screenshot 2026-09-15 at 15 46 25" src="https://github.com/user-attachments/assets/38f73264-19b6-4ffe-bb15-7d0ad4806aa7" />


## Requirements

**Your machine:**
- Go 1.24+ (to build)
- SSH access (key or agent) to every node
- Each node already trusted in `~/.ssh/known_hosts` (connect once with plain `ssh` first if not)

**Each swarm node:**
- SSH server, standard Linux `/proc`, and `df`
- `docker` CLI able to talk to the local engine (SSH user is `root` or in the `docker` group)

**Each plain server (listed under `servers`, see below):**
- SSH server, standard Linux `/proc`, and `df` — that's it, no Docker required

## Setup

1. Build:
   ```sh
   go build -o swtop ./cmd/swtop
   ```

2. Create a config file:
   ```sh
   cp config.example.yaml swtop.yaml
   ```
   ```yaml
   poll_interval: 2s

   ssh:
     user: root
     identity_file: ~/.ssh/id_rsa
     port: 22

   nodes:
     - name: manager-1
       address: 10.0.0.10
       role: manager
     - name: worker-1
       address: 10.0.0.11
       role: worker

   servers:
     - name: nas
       address: 10.0.0.20
   ```
   A node/server can override `user`, `port`, or `identity_file` individually.

   `nodes` are Docker/Swarm hosts (`docker stats`/`docker ps` are polled, containers
   show up in the UI). `servers` are plain SSH boxes monitored for resources only —
   same config shape, minus `role`, and no `docker` command is ever run against them.
   Names must be unique across both lists.

3. Run:
   ```sh
   ./swtop -config swtop.yaml
   ```
   Without `-config`, swtop looks for `~/.config/swtop/config.yaml`, then `swtop.yaml` in the current directory.

## Keys

| Key | Action |
|---|---|
| `tab` / `→` / `l` | Next view |
| `shift+tab` / `←` / `h` | Previous view |
| `0`-`9` | Jump to Cluster (`0`) or node N |
| `↑`/`↓`, `pgup`/`pgdn` | Scroll |
| `s` | Sort by CPU% or memory |
| `q` / `ctrl+c` | Quit |
