# VoxeLibre Test Runner

Automated containerised testing for the VoxeLibre project.

It consists of:

- Separate OCI images containing custom client and server builds of Luanti.
- A native Go CLI tool to automate running those builds with Docker or Podman.

## Goals

- Automated server testing on multiple Luanti versions
- Dockerized client testing on multiple Luanti versions

## Server startup tests

`vltest server unittests` starts VoxeLibre once on every supported Luanti
server and succeeds after each server reports that it is listening. Each
version has a fixed 15-second startup deadline. Test worlds and containers are
removed after every run, and the VoxeLibre clone is mounted read-only.

The default image name is a placeholder until images are published. To test a
locally built image:

```shell
docker buildx build --load \
  --file docker/server/Dockerfile \
  --tag voxelibre-test-luanti:local .

go run . server unittests \
  --voxelibre-dir ./VoxeLibre \
  --image voxelibre-test-luanti:local \
  --pull-policy never
```

Docker and Podman are supported. The default `auto` engine selection tries a
usable Docker daemon first and then Podman. Select Podman explicitly with
`--container-engine podman`.

## Configuration

By default, `vltest` searches for `vltest.json` in the working directory and
the user configuration directory. Use `--config` to select a specific file.

```json
{
  "voxelibre": {
    "clone_dir": "./VoxeLibre"
  },
  "container": {
    "engine": "auto",
    "image": "git.minetest.land/voxelibre/voxelibre-test:latest",
    "pull_policy": "missing"
  }
}
```

The equivalent flags are `--voxelibre-dir`, `--container-engine`, `--image`,
and `--pull-policy`. Environment variables use the `VLTEST_` prefix, for
example `VLTEST_VOXELIBRE_CLONE_DIR` and `VLTEST_CONTAINER_IMAGE`. Precedence
is flags, environment variables, configuration file, then defaults.

Image pull policies are:

- `missing`: use a matching local image, or pull when it is absent.
- `always`: always pull before running tests.
- `never`: require an image already present in the selected engine.

## Build the Go CLI

Will create the `vltest` binary for the host platform in the current directory.

```shell
docker bake --set vltest.output=type=local,dest=. vltest
```

```go
// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later
```
