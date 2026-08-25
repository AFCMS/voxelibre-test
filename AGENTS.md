# VoxeLibre Test Runner

## Project purpose

This repository provides containerized compatibility testing for
[VoxeLibre](https://git.minetest.land/VoxeLibre/VoxeLibre) against multiple
versions of [Luanti](https://github.com/luanti-org/luanti).

It contains two related components:

- Separate OCI images containing run-in-place client and dedicated-server
  builds for every supported Luanti version.
- The `vltest` Go CLI, which drives Docker or Podman to run compatibility tests, both on CI and developpers machines.

The current CLI commands are:

- `vltest server unittests`: mounts a local VoxeLibre clone read-only and
  verifies that every supported Luanti server reaches its listening state.
- `vltest extract-builds`: exports selected client or server build directories
  from the corresponding configured image without starting them.

The root-level `VoxeLibre/` directory is an ignored local checkout used as test
input. It is not the source code maintained by this repository. Do not modify
it unless a task explicitly asks for changes to the game checkout.

## Code layout

```text
.
├── main.go                         CLI process entry point
├── cmd/                            Cobra command definitions and wiring
│   ├── root.go                     Root command, shared flags, dependencies
│   ├── server.go                   `server` command group
│   ├── server_unittests.go         Multi-version server startup command
│   └── extractBuilds.go            Build extraction command
├── internal/
│   ├── appconfig/                  Viper defaults, flags, files, validation
│   ├── container/                  Docker/Podman-neutral interfaces and CLI backend
│   ├── luanti/                     Supported-version and build catalog
│   ├── servertest/                 Server startup lifecycle and readiness checks
│   └── buildextract/               Transactional build export lifecycle
├── docker/
│   ├── luanti/Dockerfile           Builds separate Luanti client/server images
│   └── vltest/Dockerfile           Builds the static `vltest` CLI image
├── docker-bake.hcl                 Buildx Bake definition
├── vltest.json                     Ignored local example configuration
├── vltest.schema.json              JSON Schema for configuration files
└── README.md                       User-facing setup and command examples
```

Tests live beside the packages they cover as `*_test.go` files. Keep command
handlers thin: they should validate configuration, construct dependencies,
invoke an internal runner, print results, and return errors.

## Architecture and conventions

### Cobra and Viper

- Preserve Cobra's generated registration style: define package-level command
  variables and register them with their parent command in `init()`.
- Use `RunE`, `cobra.NoArgs` where appropriate, and `cmd.Context()` for all
  potentially blocking work.
- Define flags on the command that owns them. Bind configurable flags to Viper
  and read the effective value through Viper rather than directly from a flag.
- Configuration precedence is flags, `VLTEST_` environment variables, config
  file, then defaults. Shared container settings belong on the root command.
- Keep configuration keys and defaults centralized in `internal/appconfig`.
- Keep `vltest.schema.json` synchronized whenever a Viper configuration key or
  its accepted type, values, or default changes.

### Container operations

- Business logic depends on the interfaces in `internal/container`, not on
  Docker- or Podman-specific commands.
- The current backend invokes the Docker or Podman CLI with
  `exec.CommandContext` and argument slices. Never construct shell command
  strings.
- Keep lifecycle cleanup bounded and attempt it on every success, failure, or
  cancellation path. Test worlds use anonymous volumes; the VoxeLibre checkout
  is mounted read-only.
- Docker and Podman should remain behaviorally equivalent. Add command-generation
  tests for both when extending the backend.

### Luanti builds

Builds are stored in their corresponding OCI image at:

```text
/work/dist/luanti-<version>-server
/work/dist/luanti-<version>-client
```

`internal/luanti/versions.go` is the CLI's central catalog. When adding or
removing a supported version, update all corresponding client/server stages and
copies in `docker/luanti/Dockerfile`, update the catalog, and update the tests
in the same change.
Server binaries are expected at `bin/luantiserver`; client
binaries are expected at `bin/luanti`.

The server image's persistent paths are `/var/lib/minetest` and
`/etc/minetest`. Neither image has a default entrypoint; callers select a
version-specific binary explicitly.

### Output behavior

- All automomous tests should use GitHub Action's [workflow commands](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands) where needed.
  - Group runtime logs using the `group` workflow command
  - Report erros using the `error` command

## Configuration

The main Viper keys are:

```text
voxelibre.clone_dir
container.engine
container.server_image
container.client_image
container.pull_policy
extract_builds.output_dir
```

The container engine accepts `auto`, `docker`, or `podman`. Pull policy accepts
`always`, `missing`, or `never`. See `README.md` and `vltest.schema.json` for
current defaults and examples.

## Verification

For Go changes, run:

```sh
gofmt -w <changed-go-files>
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

For Docker changes, build the relevant image when the environment permits:

```sh
docker buildx bake --load \
  --set luanti-server.tags=voxelibre-test-luanti-server:local \
  luanti-server
```

Then use `--server-image voxelibre-test-luanti-server:local --pull-policy
never` for local server CLI smoke tests. Do not create or modify CI workflow
files unless the task explicitly requests CI changes.

## Change discipline

- Preserve unrelated changes in a dirty worktree.
- New Go files should include a SPDX license header, with the Git user informations and the current year, following the convention.
- Prefer focused package tests with fake container engines; do not require a
  live Docker or Podman daemon for unit tests.
- Update user-facing documentation when commands, flags, configuration, image
  paths, or supported versions change.
