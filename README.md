# VoxeLibre Test Runner

Automated testing for the VoxeLibre project.

It consists of:

- Separate OCI images containing custom client and server builds of Luanti, as well as other tools.
- A native Go CLI tool to automate running those builds with Docker or Podman.

## Features

- [x] OCI images
  - [x] Server builds
  - [x] Client builds
  - [x] Tools (LuaLS, etc)
- [ ] CLI
  - [x] Automated Docker server startup test with GitHub Actions compatible annotations
    - [ ] Unittests?
    - [ ] Test client connection?
  - [x] Client standalone builds extraction
  - [ ] Client automated startup
    - [x] Native (extraction to temp file system)
    - [ ] Docker + VNC + Web UI
    - [ ] Docker + native Wayland socket
    - [ ] Docker + native X11 socket
    - [ ] Docker GPU acceleration
  - [x] Linting automation (CI + local)
    - [x] LuaLS
  - [x] Config file system
- [ ] Distribution
  - [x] Docker CI
  - [ ] CLI CI
- [ ] User + Agent documentation

## System requirements

- Linux x86-64.
- Docker Engine 28 or newer, or Podman 5 or newer.
- Permission to run containers and a local VoxeLibre checkout.
- Native clients and extracted builds require glibc 2.39 or newer.
- Native clients require a working graphical desktop and audio stack.
- Local image builds require Docker Buildx 0.28 or newer and internet access.

All commands need Docker or Podman. Native client sessions run Luanti on the
host, but use the container runtime to extract the selected build.

## Configuration

By default, `vltest` searches for `vltest.json` in the working directory and
the user configuration directory. Use `--config` to select a specific file.
See [`vltest.schema.json`](vltest.schema.json) for every setting.

Configuration precedence is flags, `VLTEST_` environment variables, the
configuration file, then defaults. The default `auto` engine tries Docker and
then Podman.

## Typical use

### Test server startup

Test every supported Luanti server against the configured VoxeLibre checkout:

```shell
vltest server unittests
```

### Lint VoxeLibre with LuaLS

Run LuaLS and emit GitHub Actions compatible annotations for errors and
warnings:

```shell
vltest lint
```

Only report error-level diagnostics:

```shell
vltest lint --check-level error
```

The accepted minimum levels are `error`, `warning`, `information`, and `hint`.
Information and hint diagnostics are emitted as notices. Error diagnostics make
the command fail; warnings and notices do not.

### Launch a native client

Open the Luanti main menu with an ephemeral profile:

```shell
vltest client native --version 5.17.0
```

Start VoxeLibre immediately and retain the profile:

```shell
vltest client native \
  --version 5.17.0 \
  --start-world \
  --data-dir ~/.local/share/vltest
```

Supported versions are `5.14.0`, `5.15.2`, `5.16.1`, and `5.17.0`.

### Extract builds

Extract one client build to `./builds`:

```shell
vltest extract-builds --version 5.17.0 --kind client
```

Extract every server and client build:

```shell
vltest extract-builds --all
```

Extraction never overwrites an existing build directory.

## Agent skill

An [agent skill](https://agentskills.io) is provided to provide instructions for AI agents to use VoxeLibre testing stack.

You can find it in `.agents/skills/vltest`, or install it using the [`skills`](https://www.skills.sh) CLI:

```shell
npx skills add https://git.minetest.land/VoxeLibre/voxelibre-test --skill vltest

# Update installed skills
npx skills update
```

## Docker images

Manually dispatch the `Docker Images` Forgejo workflow to publish:

- `git.minetest.land/voxelibre/voxelibre-test/luanti-client`
- `git.minetest.land/voxelibre/voxelibre-test/luanti-server`
- `git.minetest.land/voxelibre/voxelibre-test/tools`

## Build locally

From the repository root, build the CLI and container images:

```shell
docker buildx bake \
  --set vltest.output=type=local,dest=. \
  vltest

docker buildx bake --load \
  --set luanti-server.tags=voxelibre-test-luanti-server:local \
  luanti-server

docker buildx bake --load \
  --set luanti-client.tags=voxelibre-test-luanti-client:local \
  luanti-client

docker buildx bake --load \
  --set tools.tags=voxelibre-test-tools:local \
  tools
```

Add the generated `vltest` binary to your `PATH`, then create `vltest.json`:

```json
{
  "voxelibre": {
    "clone_dir": "./VoxeLibre"
  },
  "container": {
    "engine": "docker",
    "server_image": "voxelibre-test-luanti-server:local",
    "client_image": "voxelibre-test-luanti-client:local",
    "tools_image": "voxelibre-test-tools:local",
    "pull_policy": "never"
  },
  "client": {
    "data_dir": "./.vltest/client"
  },
  "extract_builds": {
    "output_dir": "./builds"
  },
  "lint": {
    "check_level": "warning"
  }
}
```
