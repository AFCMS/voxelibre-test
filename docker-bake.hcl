// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

target "docker-metadata-action" {}

target "vltest" {
    context = "."
    dockerfile = "docker/Dockerfile"
}