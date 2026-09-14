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
  - [x] Automated Lua unit tests across Luanti servers
    - [x] Synchronous mod-owned unit tests
    - [ ] Test client connection?
  - [x] Client standalone builds extraction
  - [x] Native client startup (extraction to temp file system)
  - [x] Linting automation (CI + local)
    - [x] LuaLS
  - [x] Config file system
- [x] Distribution
  - [x] Docker CI
  - [x] CLI CI
- [x] User + Agent documentation

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

The VoxeLibre clone directory defaults to the current working directory (`.`).
Run commands from your VoxeLibre checkout, or set `--voxelibre-dir` or
`voxelibre.clone_dir` to use another location.

Configuration precedence is flags, `VLTEST_` environment variables, the
configuration file, then defaults. The default `auto` engine tries Docker and
then Podman.

### Running inside CI job containers

`vltest lint` and `vltest server unittests` can reuse the checkout mounted
inside a Forgejo Actions job container. With the default
`container.workspace_container: "auto"`, `vltest` detects Docker/Podman container
marker files and inspects the container identified by its hostname using the
selected engine. The job and test containers must use the same daemon.

Workspace volumes are reused by name and mounted read-only; bind-mounted
workspaces are translated to their daemon-side paths. The local checkout path
continues to be used for validation and diagnostic annotations. The runner's
workspace volume is never removed by `vltest` cleanup.

If the job uses a custom hostname, select its actual container ID or name:

```shell
vltest --workspace-container my-job lint
```

The equivalent environment variable is `VLTEST_CONTAINER_WORKSPACE_CONTAINER`.
Set `--workspace-container none` to use checkout paths directly on the daemon,
including when container detection is inappropriate for your setup. On an
ordinary host, `auto` leaves bind mounts unchanged. Build extraction and native
client commands do not inspect workspace mounts.

Checkouts below a volume root require volume subdirectory support in both the
engine and CLI (Docker 26+, or a Podman release supporting `--mount ... subpath`).
An unsupported engine reports a volume subdirectory startup error. Checkouts in
the job container's writable layer, unsupported mount types such as tmpfs, and
checkouts containing additional nested mounts are rejected with an explanation.

## Typical use

### Run VoxeLibre unit tests

Run mod-owned Lua tests against every supported Luanti server:

```shell
vltest server unittests
```

The command run servers sequentially, enabling `vl_unittests_enabled`, then collects the unittests results. JSON results are transformed into a human-readable summary by the CLI.

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

The separate `Docker vltest` Forgejo workflow builds the `vltest` image for
pull requests to `master`, pushes to `master`, tags, and manual dispatches.
Only pushes to `master` and tags publish to
`git.minetest.land/voxelibre/voxelibre-test/vltest`, using the branch or tag name
as the image tag.

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

Add the generated `vltest` binary to your `PATH`, then create `vltest.json`
in your VoxeLibre checkout:

```json
{
  "voxelibre": {
    "clone_dir": "."
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

## Maintenance

### Add a Luanti version

1. In [`docker/luanti/Dockerfile`](docker/luanti/Dockerfile), duplicate an existing
   version's source, client, and server stages. Update the version references and
   add both builds to the final images' `COPY` instructions.
2. Add the version to [`internal/luanti/versions.go`](internal/luanti/versions.go)
   and update its tests and the supported-version list above.
3. Run `gofmt`, `go test ./...`, `go test -race ./...`, `go vet ./...`, and
   `git diff --check`.
4. [Rebuild the CLI and both images](#build-locally). Run server unit tests,
   extract the new builds, and launch the new native client using those images.
5. After merging, manually dispatch **Docker Images** in Forgejo Actions on
   `master` to publish the updated Luanti images (no inputs required). The
   **Docker vltest** workflow publishes the CLI image automatically on pushes
   to `master` or tags; manually dispatching it only builds the image.
