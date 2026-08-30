# syntax=docker/dockerfile:1
# check=error=true
# SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
# SPDX-License-Identifier: LGPL-3.0-or-later

# We have to ignore some dockerfile-utils errors, because `--unpack` is currently unsupported for the `ADD` instruction
# - https://github.com/microsoft/vscode-containers/issues/297
# - https://github.com/rcjsuen/dockerfile-utils/issues/131

FROM gcr.io/distroless/base-nossl-debian13:nonroot AS base

WORKDIR /tmp

ENV LUALS_VERSION=3.19.1
ENV LUALS_CHECKSUM=sha256:e9235d2d72ef55bc41cf8c99cda2ed64777682024b4bb81f5dea425060c5cbb8

# dockerfile-utils: ignore
ADD --unpack=true \
    --checksum=${LUALS_CHECKSUM} \
    https://github.com/LuaLS/lua-language-server/releases/download/${LUALS_VERSION}/lua-language-server-${LUALS_VERSION}-linux-x64.tar.gz /opt/LuaLS
