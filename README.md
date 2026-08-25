# VoxeLibre Test Runner

Automated containerised testing for the VoxeLibre project.

It consists of:

- Separate OCI images containing custom client and server builds of Luanti.
- A native Go CLI tool to automate running those builds with Docker or Podman.

## Goals

- Automated server testing on multiple Luanti versions
- Dockerized client testing on multiple Luanti versions

## Usage

**Validate server startup (for use in CI):**

```shell
vltest server unittests
```

## Extracting Luanti builds

`vltest extract-builds` copies complete run-in-place build directories from the
configured images without starting a container. Select exactly one version or
all versions; `--kind` independently selects server builds, client builds, or
both. For example:

```shell
# Both builds for one version
go run . extract-builds --version 5.16.1

# One client build in a chosen directory
go run . extract-builds \
  --version 5.16.1 \
  --kind client \
  --output-dir ./artifacts

# Every supported server build
go run . extract-builds --all --kind server

# Every supported server and client build
go run . extract-builds --all
```

The output directory defaults to `./builds`. Each image directory keeps its
name, such as `builds/luanti-5.16.1-client`. Extraction refuses to overwrite or
merge an existing build directory. Server-only extraction resolves and pulls
only the server image; client-only extraction does the same for the client
image. Both images are needed only when both build kinds are selected.

Each extracted build is self-contained except for glibc and the host's hardware
integration. Its non-glibc shared libraries are stored in `lib/`, and the
executable uses an `$ORIGIN`-relative runtime search path. Keep `bin/` and
`lib/` together; matching Debian packages and `LD_LIBRARY_PATH` are not needed.
The current x86-64 builds require glibc 2.39 or newer. Client builds still need
a working host graphics/audio stack and access to the corresponding devices and
display/audio sockets.

## Server startup tests

`vltest server unittests` starts VoxeLibre once on every supported Luanti
server and succeeds after each server reports that it is listening. Each
version has a fixed 15-second startup deadline. Test worlds and containers are
removed after every run, and the VoxeLibre clone is mounted read-only.

The default image names are placeholders until images are published. To build
and test the server image locally:

```shell
docker bake luanti-server --load \
  --set luanti-server.tags=voxelibre-test-luanti-server:local

go run . server unittests \
  --voxelibre-dir ./VoxeLibre \
  --server-image voxelibre-test-luanti-server:local \
  --pull-policy never
```

The server command never resolves or pulls the client image. Build both images
for extraction of both build kinds:

```shell
docker bake luanti --load \
  --set luanti-server.tags=voxelibre-test-luanti-server:local \
  --set luanti-client.tags=voxelibre-test-luanti-client:local
```

Docker and Podman are supported. The default `auto` engine selection tries a
usable Docker daemon first and then Podman. Select Podman explicitly with
`--container-engine podman`.

## Configuration

By default, `vltest` searches for `vltest.json` in the working directory and
the user configuration directory. Use `--config` to select a specific file.
The configuration format is described by [`vltest.schema.json`](vltest.schema.json).

```json
{
  "voxelibre": {
    "clone_dir": "./VoxeLibre"
  },
  "container": {
    "engine": "auto",
    "server_image": "git.minetest.land/voxelibre/voxelibre-test/luanti-server:latest",
    "client_image": "git.minetest.land/voxelibre/voxelibre-test/luanti-client:latest",
    "pull_policy": "missing"
  },
  "extract_builds": {
    "output_dir": "./builds"
  }
}
```

The equivalent flags are `--voxelibre-dir`, `--container-engine`,
`--server-image`, `--client-image`, and `--pull-policy`. Environment variables
use the `VLTEST_` prefix, for example `VLTEST_VOXELIBRE_CLONE_DIR`,
`VLTEST_CONTAINER_SERVER_IMAGE`, and `VLTEST_CONTAINER_CLIENT_IMAGE`.
Precedence is flags, environment variables, configuration file, then defaults.

The extraction output directory can also be set with
`VLTEST_EXTRACT_BUILDS_OUTPUT_DIR`. Version and build-kind selection remain
explicit command options.

Image pull policies are:

- `missing`: use a matching local image, or pull when it is absent.
- `always`: always pull before running tests.
- `never`: require an image already present in the selected engine.

## Build the Go CLI

Will create the `vltest` binary for the host platform in the current directory.

```shell
docker bake --set vltest.output=type=local,dest=. vltest
```
