# VoxeLibre Test Runner

Automated containerised testing for the VoxeLibre project.

## Goals

- Automated server testing on multiple Luanti versions
- Dockerized client testing on multiple Luanti versions

## Build the Go CLI

Will create the `vltest` binary for the host platform in the current directory.

```shell
docker build -f docker/Dockerfile --output type=local,dest=. .
```
