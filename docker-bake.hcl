// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

target "docker-metadata-action" {}

target "vltest" {
    context = "."
    dockerfile = "docker/Dockerfile"
}

target "luanti-client" {
    context = "."
    dockerfile = "docker/luanti/Dockerfile"
    target = "luanti-clients"
}

target "luanti-server" {
    context = "."
    dockerfile = "docker/luanti/Dockerfile"
    target = "luanti-servers"
}

group "luanti" {
    targets = ["luanti-client", "luanti-server"]
}
