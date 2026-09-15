# swtop

[![Release](https://github.com/dbohry/swtop/actions/workflows/release.yml/badge.svg)](https://github.com/dbohry/swtop/actions/workflows/release.yml)
[![Latest Release](https://img.shields.io/github/v/release/dbohry/swtop)](https://github.com/dbohry/swtop/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/dbohry/swtop)](go.mod)

A `btop`/`htop`-style terminal UI for a Docker Swarm cluster. Connects to every node over SSH and shows a consolidated cluster view plus a per-node view (CPU, memory, disk, network, and containers).

## Requirements

**Your machine:**
- Go 1.24+ (to build)
- SSH access (key or agent) to every node
- Each node already trusted in `~/.ssh/known_hosts` (connect once with plain `ssh` first if not)

**Each swarm node:**
- SSH server, standard Linux `/proc`, and `df`
- `docker` CLI able to talk to the local engine (SSH user is `root` or in the `docker` group)

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
   ```
   A node can override `user`, `port`, or `identity_file` individually.

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
