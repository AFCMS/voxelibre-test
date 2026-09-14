---
name: vltest
description: Run and diagnose VoxeLibre tests and LuaLS linting, launch native Luanti clients, extract builds, and write mod-owned Lua tests with vltest. For CLI usage, not Go runner or container-image maintenance.
license: LGPL-3.0-or-later
---

# VoxeLibre testing with vltest

## Setup

- Use `vltest` on `PATH` or an existing local binary. Check subcommand `--help`
  for installed capabilities and choose a version supported by that installation.
- Run from the VoxeLibre checkout, or pass `--voxelibre-dir /path/to/VoxeLibre`.
  The default checkout is `.`.
- Configuration precedence is flags, `VLTEST_` environment variables, config
  file, then defaults. The CLI looks for `vltest.json` in the working or user
  config directory; `--config FILE` selects a file explicitly. Inspect existing
  settings before overriding them, and prefer flags for a one-off run.
- Commands require access to Docker or Podman. `--container-engine auto`
  tries Docker then Podman; `docker` and `podman` select one explicitly.
  `--server-image`, `--client-image`, and `--tools-image` select the relevant
  images. Pull policy `--pull-policy` accepts `missing` (default), `always`, or `never`.

## Run checks and interpret results

### Lua unit tests

```sh
vltest --voxelibre-dir /path/to/VoxeLibre server unittests
```

Runs every supported server sequentially with fresh worlds, a read-only
checkout, and a 60-second startup-to-shutdown deadline per version. Individual
failures allow later versions to run. The server image's
`/etc/minetest/unittests.conf` enables the `vl_unittests_enabled` boolean setting.

Success requires zero server exit status, a complete consistent nonempty suite,
and no failed cases. Invalid/missing summaries and older checkouts without the
framework fail. Luanti exits gracefully even after assertions fail: use
**vltest's exit status**, not server shutdown or listening, to judge success.

Luanti emits `[vl_unittests] ` JSON records; vltest formats results and preserves
engine logs.

### LuaLS linting

```sh
vltest lint
vltest lint --check-level error
```

Minimum severity defaults to `warning`; levels are `error`, `warning`,
`information`, and `hint`. Errors and infrastructure failures fail the command;
warnings/notices alone do not. Lint emits workflow annotations. Report remaining
warnings separately from the exit status.

### Native clients and extraction

Replace `VERSION` with a supported Luanti version:

```sh
vltest client native --version VERSION
vltest client native --version VERSION --start-world --data-dir /path/to/profile
vltest client native --version VERSION -- --verbose
vltest extract-builds --version VERSION --kind client --output-dir ./builds
vltest extract-builds --all
```

Native clients can run on a Linux x86-64 host using extracted builds; they need a
desktop, audio libraries, and glibc 2.39+. Without a configured `data-dir`, the
profile is temporary; otherwise profiles persist per version and are locked
while used. `--start-world` opens VoxeLibre immediately. Forward Luanti arguments
after `--`, avoiding conflicting world selectors. Report actual gameplay checks,
not just successful launch.

Extraction requires either `--version` or `--all`; `--kind` accepts `client`,
`server`, or `all`. Existing build directories are never overwritten or merged:
choose another output directory on collision. Preserve persistent client profiles.

## Write mod-owned Lua tests

Read `mods/CORE/vl_unittests/API.md` and existing tests in the target checkout
when available. Add `vl_unittests` to the mod's `depends` in `mod.conf`, preserving
existing dependencies. Explicitly load a test file from `init.lua` when enabled:

```lua
if vl_unittests.enabled then
    dofile(core.get_modpath(core.get_current_modname()) .. "/tests.lua")
end
```

For example, in `mcl_util/tests.lua`:

```lua
local t = vl_unittests

t.test("roman numeral conversion", function()
    t.assert_equal(mcl_util.to_roman(4), "IV")
end)
```

`test(name, func)` captures the registering mod. Names must contain non-whitespace
text and be unique within the mod. Register during loading; there is no stage
argument or scanning. Enabled inline registration can capture local functions.

Tests run synchronously at `on_mods_loaded`, sorted by mod/name. Errors fail;
return values are ignored. Failures capture tracebacks without stopping the suite.
The enabled suite emits its summary and requests shutdown; the setting defaults
to false for ordinary game sessions.

| Helper                                               | Meaning                                                                                                     |
| ---------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `assert_equal(actual, expected, message?)`           | Ordinary Lua equality, including identity semantics for ordinary tables.                                    |
| `assert_deep_equal(actual, expected, message?)`      | Compare raw nested contents, handling cycles and ignoring metatables; table keys retain identity semantics. |
| `assert_near(actual, expected, tolerance, message?)` | Absolute numeric difference within an explicit finite, nonnegative tolerance.                               |
| `assert_error(func, expected_substring?, message?)`  | Require an error, optionally containing literal text, not a Lua pattern.                                    |

Helpers raise diagnostics on failure; ordinary Lua `assert` also works. The
optional final message adds context. To supply only a custom message to
`assert_error`, pass `nil` for the substring.
