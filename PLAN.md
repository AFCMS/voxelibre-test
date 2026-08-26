# Client Startup: Native, Container, VNC, Wayland, and X11

## Summary

Add interactive client launching for one explicit Luanti version:

```text
vltest client native --version VERSION [--data-dir DIR] [--start-world] [-- LUANTI_ARGS...]
vltest client container --version VERSION [OPTIONS] [-- LUANTI_ARGS...]
```

- Default launch opens Luanti’s main menu; `--start-world` enters a per-version VoxeLibre world immediately.
- Sessions are ephemeral unless `--data-dir` enables a persistent, version-isolated profile shared by native and container launches.
- Support Docker and Podman equally. Client startup is Linux/x86-64 only; native execution retains the existing glibc 2.39 requirement.

## CLI and Configuration Contract

`client container` adds:

- `--display auto|wayland|x11|vnc|web`, default `auto`. Auto selects a valid Wayland socket first, then local X11, otherwise returns an actionable error.
- `--gpu auto|mesa|nvidia|software`, default `auto`.
- `--audio auto|none`, default `auto`; unavailable Pulse/PipeWire-Pulse audio warns but does not block launch.
- `--resolution 1280x720`, `--vnc-port 5900`, and `--web-port 6080`.
- Raw VNC publishes only `127.0.0.1:5900`; web mode publishes only `127.0.0.1:6080`, keeping RFB internal.
- VNC/web generate and print a one-session password. vltest prints the endpoint but never launches a viewer or browser.
- Arbitrary Luanti arguments require `--`. With `--start-world`, reject conflicting `--world`, `--worldname`, `--gameid`, or `--go` arguments.

Add schema-backed settings:

```text
client.data_dir
client.container.display
client.container.gpu
client.container.audio
client.container.vnc.resolution
client.container.vnc.port
client.container.vnc.web_port
```

Version and `--start-world` remain explicit invocation choices.

## Implementation Changes

- Add thin Cobra commands plus an `internal/clientrun` package containing independently testable native and container runners. Refactor VoxeLibre clone validation for reuse by server and client settings.
- Native sessions reuse the transactional build-extraction runner. Ephemeral mode extracts into `os.MkdirTemp`, launches the run-in-place binary with inherited stdio/environment, and always removes the directory. Persistent mode transactionally creates `<data-dir>/luanti-VERSION-client`, locks it against concurrent use, and reuses it until the user removes that version directory.
- Expose the arbitrary checkout name as game ID `voxelibre` through a temporary game-path symlink and both legacy/current Luanti environment variables. `--start-world` uses `worlds/vltest`, retained only for persistent profiles.
- Extend the neutral `ContainerSpec` with typed environment variables, published ports, host-device mappings, host UID/GID and supplementary groups, and semantic GPU intent. Generate deterministic, shell-free Docker and Podman arguments.
- Run graphical containers as the invoking user. Make image-resident client trees writable for ephemeral overlay state; mount persistent profiles at the matching build path. Preserve detached start/wait/log behavior, bounded stop/removal, nonzero exit reporting, and cleanup after cancellation.
- Wayland mounts only the validated display socket and forces `SDL_VIDEODRIVER=wayland`. X11 accepts local displays only, mounts the exact socket, creates a scoped temporary authority file using host `xauth`, and forces `SDL_VIDEODRIVER=x11`; never use `xhost`.
- Direct-display audio mounts only the detected Pulse/PipeWire-Pulse socket and optional cookie. VNC/web explicitly provide no remote audio.
- Mesa passes accessible DRM render nodes and supplementary groups. NVIDIA uses Docker `--gpus all` or Podman CDI `--device=nvidia.com/gpu=all`, with `NVIDIA_DRIVER_CAPABILITIES=graphics,display,utility`.
- VNC/web allow Mesa DRI3 acceleration or software rendering. Reject explicit NVIDIA because TigerVNC still has an unresolved proprietary-driver fallback issue; NVIDIA remains supported for Wayland/X11. [TigerVNC render-node documentation](https://github.com/TigerVNC/tigervnc/blob/master/unix/xserver/hw/vnc/Xvnc.man), [upstream NVIDIA issue](https://github.com/TigerVNC/tigervnc/issues/1773).
- Keep one client image. Add TigerVNC, Tini, a minimal window manager, and `python3-websockify`; copy pinned noVNC 1.7.0 static assets instead of installing Debian’s Node-dependent noVNC package. Python remains because upstream identifies it as the supported WebSocket bridge implementation. [noVNC requirements](https://github.com/novnc/noVNC), [websockify implementation matrix](https://github.com/novnc/websockify/wiki/Feature_Matrix).
- Add a supervised VNC launcher that creates the TigerVNC password file from a mounted secret, starts X/TWM/websockify as required, emits a readiness marker, forwards signals, reaps children, and returns Luanti’s exit status.

## Documentation and Tests

- Expand README examples for native, auto-world, persistence, Wayland, X11, raw VNC, web UI, Mesa, and NVIDIA. Document localhost-only endpoints, X11/Wayland trust implications, `xauth`, glibc, DRM group/SELinux requirements, and lack of VNC audio.
- Link current NVIDIA installation/configuration guidance for [Docker and Podman](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html) and runtime verification examples from the [NVIDIA sample workload guide](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/sample-workload.html).
- Unit-test CLI registration, flag/config/schema validation, argument pass-through/conflicts, temporary cleanup, persistent profile reuse/locking, start-world behavior, display/audio discovery, VNC readiness/password output, exit/cancellation cleanup, and every display/GPU specification.
- Add exact Docker and Podman command-generation tests, including Mesa devices, Docker NVIDIA, Podman CDI, identity/groups, socket mounts, ports, and unchanged server behavior.
- Smoke-test the image in software VNC and noVNC modes; test Mesa rendering against `/dev/dri`, NVIDIA on equipped Wayland/X11 hosts, and manually verify both direct-display paths.
- Fix the existing stale three-version assertions so tests derive expectations from the four-version catalog, then run gofmt, `go test ./...`, race tests, vet, `git diff --check`, and the client-image build. Do not modify CI workflows or unrelated untracked files.
