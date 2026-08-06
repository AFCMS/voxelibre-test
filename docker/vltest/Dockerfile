# syntax=docker/dockerfile:1
# check=error=true
# SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
# SPDX-License-Identifier: LGPL-3.0-or-later

FROM --platform=$BUILDPLATFORM golang:1.26-alpine3.24 AS builder-base

ARG TARGETOS
ARG TARGETARCH

ENV GOCACHE=/root/.cache/go-build

# Install build dependencies
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN --mount=type=cache,id=gomod,target="/go/pkg/mod" go mod download
RUN --mount=type=cache,id=gomod,target="/go/pkg/mod" go mod verify

COPY . ./

# Build with cache
# https://dev.to/jacktt/20x-faster-golang-docker-builds-289n
RUN --mount=type=cache,id=gomod,target="/go/pkg/mod" \
    --mount=type=cache,id=gobuild,target="/root/.cache/go-build" \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o vltest

FROM scratch AS base

COPY --from=builder-base /app/vltest /vltest

ENTRYPOINT ["/vltest"]