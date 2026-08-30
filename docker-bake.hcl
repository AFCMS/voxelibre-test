// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

target "docker-metadata-action-luanti-client" {}

target "docker-metadata-action-luanti-server" {}

target "docker-metadata-action-tools" {}

target "vltest" {
    context = "."
    dockerfile = "docker/vltest/Dockerfile"
}

target "tools" {
    inherits = ["docker-metadata-action-tools"]
    context = "."
    dockerfile = "docker/tools/Dockerfile"
}

target "luanti-client" {
    inherits = ["docker-metadata-action-luanti-client"]
    context = "."
    dockerfile = "docker/luanti/Dockerfile"
    target = "luanti-clients"
}

target "luanti-server" {
    inherits = ["docker-metadata-action-luanti-server"]
    context = "."
    dockerfile = "docker/luanti/Dockerfile"
    target = "luanti-servers"
}

group "luanti" {
    targets = ["luanti-client", "luanti-server"]
}

group "images" {
    targets = ["luanti-client", "luanti-server", "tools"]
}
