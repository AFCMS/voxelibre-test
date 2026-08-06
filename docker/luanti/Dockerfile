# syntax=docker/dockerfile:1
# check=error=true
# SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
# SPDX-License-Identifier: LGPL-3.0-or-later

ARG DEBIAN_VERSION=trixie-slim
ARG BUILD_JOBS=2

FROM debian:${DEBIAN_VERSION} AS build-base

ARG BUILD_JOBS
ARG LUAJIT_REF=v2.1

ENV CMAKE_BUILD_PARALLEL_LEVEL=${BUILD_JOBS}

RUN apt-get update && \
    apt-get install --yes --no-install-recommends \
    build-essential \
    ca-certificates \
    cmake \
    gettext \
    git \
    libcurl4-openssl-dev \
    libfreetype-dev \
    libgl1-mesa-dev \
    libgmp-dev \
    libhiredis-dev \
    libjpeg-dev \
    libjsoncpp-dev \
    libleveldb-dev \
    libogg-dev \
    libopenal-dev \
    libpng-dev \
    libpq-dev \
    libsdl2-dev \
    libspatialindex-dev \
    libsqlite3-dev \
    libvorbis-dev \
    libzstd-dev \
    ninja-build \
    zlib1g-dev && \
    rm -rf /var/lib/apt/lists/*

ADD --keep-git-dir https://luajit.org/git/luajit.git?branch=${LUAJIT_REF} /usr/src/luajit

RUN make -C /usr/src/luajit amalg && \
    make -C /usr/src/luajit install


FROM build-base AS bundle-tools

RUN apt-get update && \
    apt-get install --yes --no-install-recommends patchelf && \
    rm -rf /var/lib/apt/lists/*

COPY --chmod=0755 docker/luanti/bundle-libraries.sh /usr/local/bin/bundle-libraries


FROM build-base AS luanti-5.16.1-source

ADD https://github.com/luanti-org/luanti.git?tag=5.16.1 /work/src/luanti


FROM luanti-5.16.1-source AS luanti-5.16.1-client

WORKDIR /work/src/luanti

RUN cmake -S . -B build-client -G Ninja \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_INSTALL_PREFIX=/work/dist/luanti-5.16.1-client \
    -DRUN_IN_PLACE=TRUE \
    -DBUILD_CLIENT=TRUE \
    -DBUILD_SERVER=FALSE \
    -DBUILD_UNITTESTS=FALSE \
    -DBUILD_BENCHMARKS=FALSE \
    -DBUILD_DOCUMENTATION=FALSE \
    -DENABLE_PROMETHEUS=FALSE \
    -DENABLE_SPATIAL=TRUE && \
    cmake --build build-client && \
    cmake --install build-client

RUN --mount=from=bundle-tools,source=/usr/bin/patchelf,target=/usr/local/bin/patchelf,ro \
    --mount=from=bundle-tools,source=/usr/local/bin/bundle-libraries,target=/usr/local/bin/bundle-libraries,ro \
    LD_LIBRARY_PATH=/usr/local/lib \
    bundle-libraries /work/dist/luanti-5.16.1-client bin/luanti


FROM luanti-5.16.1-source AS luanti-5.16.1-server

WORKDIR /work/src/luanti

RUN cmake -S . -B build-server -G Ninja \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_INSTALL_PREFIX=/work/dist/luanti-5.16.1-server \
    -DRUN_IN_PLACE=TRUE \
    -DBUILD_CLIENT=FALSE \
    -DBUILD_SERVER=TRUE \
    -DBUILD_UNITTESTS=FALSE \
    -DBUILD_BENCHMARKS=FALSE \
    -DBUILD_DOCUMENTATION=FALSE \
    -DENABLE_PROMETHEUS=FALSE \
    -DENABLE_SPATIAL=TRUE && \
    cmake --build build-server && \
    cmake --install build-server

RUN --mount=from=bundle-tools,source=/usr/bin/patchelf,target=/usr/local/bin/patchelf,ro \
    --mount=from=bundle-tools,source=/usr/local/bin/bundle-libraries,target=/usr/local/bin/bundle-libraries,ro \
    LD_LIBRARY_PATH=/usr/local/lib \
    bundle-libraries /work/dist/luanti-5.16.1-server bin/luantiserver


FROM build-base AS luanti-5.15.2-source

ADD https://github.com/luanti-org/luanti.git?tag=5.15.2 /work/src/luanti


FROM luanti-5.15.2-source AS luanti-5.15.2-client

WORKDIR /work/src/luanti

RUN cmake -S . -B build-client -G Ninja \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_INSTALL_PREFIX=/work/dist/luanti-5.15.2-client \
    -DRUN_IN_PLACE=TRUE \
    -DBUILD_CLIENT=TRUE \
    -DBUILD_SERVER=FALSE \
    -DBUILD_UNITTESTS=FALSE \
    -DBUILD_BENCHMARKS=FALSE \
    -DBUILD_DOCUMENTATION=FALSE \
    -DENABLE_PROMETHEUS=FALSE \
    -DENABLE_SPATIAL=TRUE && \
    cmake --build build-client && \
    cmake --install build-client

RUN --mount=from=bundle-tools,source=/usr/bin/patchelf,target=/usr/local/bin/patchelf,ro \
    --mount=from=bundle-tools,source=/usr/local/bin/bundle-libraries,target=/usr/local/bin/bundle-libraries,ro \
    LD_LIBRARY_PATH=/usr/local/lib \
    bundle-libraries /work/dist/luanti-5.15.2-client bin/luanti


FROM luanti-5.15.2-source AS luanti-5.15.2-server

WORKDIR /work/src/luanti

RUN cmake -S . -B build-server -G Ninja \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_INSTALL_PREFIX=/work/dist/luanti-5.15.2-server \
    -DRUN_IN_PLACE=TRUE \
    -DBUILD_CLIENT=FALSE \
    -DBUILD_SERVER=TRUE \
    -DBUILD_UNITTESTS=FALSE \
    -DBUILD_BENCHMARKS=FALSE \
    -DBUILD_DOCUMENTATION=FALSE \
    -DENABLE_PROMETHEUS=FALSE \
    -DENABLE_SPATIAL=TRUE && \
    cmake --build build-server && \
    cmake --install build-server

RUN --mount=from=bundle-tools,source=/usr/bin/patchelf,target=/usr/local/bin/patchelf,ro \
    --mount=from=bundle-tools,source=/usr/local/bin/bundle-libraries,target=/usr/local/bin/bundle-libraries,ro \
    LD_LIBRARY_PATH=/usr/local/lib \
    bundle-libraries /work/dist/luanti-5.15.2-server bin/luantiserver


FROM build-base AS luanti-5.14.0-source

ADD https://github.com/luanti-org/luanti.git?tag=5.14.0 /work/src/luanti


FROM luanti-5.14.0-source AS luanti-5.14.0-client

WORKDIR /work/src/luanti

RUN cmake -S . -B build-client -G Ninja \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_INSTALL_PREFIX=/work/dist/luanti-5.14.0-client \
    -DRUN_IN_PLACE=TRUE \
    -DBUILD_CLIENT=TRUE \
    -DBUILD_SERVER=FALSE \
    -DBUILD_UNITTESTS=FALSE \
    -DBUILD_BENCHMARKS=FALSE \
    -DBUILD_DOCUMENTATION=FALSE \
    -DENABLE_PROMETHEUS=FALSE \
    -DENABLE_SPATIAL=TRUE && \
    cmake --build build-client && \
    cmake --install build-client

RUN --mount=from=bundle-tools,source=/usr/bin/patchelf,target=/usr/local/bin/patchelf,ro \
    --mount=from=bundle-tools,source=/usr/local/bin/bundle-libraries,target=/usr/local/bin/bundle-libraries,ro \
    LD_LIBRARY_PATH=/usr/local/lib \
    bundle-libraries /work/dist/luanti-5.14.0-client bin/luanti


FROM luanti-5.14.0-source AS luanti-5.14.0-server

WORKDIR /work/src/luanti

RUN cmake -S . -B build-server -G Ninja \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_INSTALL_PREFIX=/work/dist/luanti-5.14.0-server \
    -DRUN_IN_PLACE=TRUE \
    -DBUILD_CLIENT=FALSE \
    -DBUILD_SERVER=TRUE \
    -DBUILD_UNITTESTS=FALSE \
    -DBUILD_BENCHMARKS=FALSE \
    -DBUILD_DOCUMENTATION=FALSE \
    -DENABLE_PROMETHEUS=FALSE \
    -DENABLE_SPATIAL=TRUE && \
    cmake --build build-server && \
    cmake --install build-server

RUN --mount=from=bundle-tools,source=/usr/bin/patchelf,target=/usr/local/bin/patchelf,ro \
    --mount=from=bundle-tools,source=/usr/local/bin/bundle-libraries,target=/usr/local/bin/bundle-libraries,ro \
    LD_LIBRARY_PATH=/usr/local/lib \
    bundle-libraries /work/dist/luanti-5.14.0-server bin/luantiserver


FROM debian:${DEBIAN_VERSION} AS runtime-base

RUN apt-get update && \
    apt-get install --yes --no-install-recommends \
    ca-certificates \
    libcurl4t64 \
    libgmp10 \
    libhiredis1.1.0 \
    libjsoncpp26 \
    libleveldb1d \
    libpq5 \
    libspatialindex8 \
    libsqlite3-0 \
    libstdc++6 \
    libzstd1 \
    zlib1g && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p \
    /etc/minetest \
    /var/lib/minetest/games \
    /var/lib/minetest/mods \
    /var/lib/minetest/worlds

RUN --mount=from=build-base,source=/usr/local/lib,target=/mnt/luajit,ro \
    cp --archive /mnt/luajit/libluajit-5.1.so* /usr/local/lib/ && \
    ldconfig

ENV LUANTI_GAME_PATH=/var/lib/minetest/games \
    LUANTI_MOD_PATH=/var/lib/minetest/mods \
    LUANTI_WORLD_PATH=/var/lib/minetest/worlds \
    MINETEST_GAME_PATH=/var/lib/minetest/games \
    MINETEST_MOD_PATH=/var/lib/minetest/mods \
    MINETEST_WORLD_PATH=/var/lib/minetest/worlds

WORKDIR /var/lib/minetest

USER root

VOLUME ["/var/lib/minetest", "/etc/minetest"]


FROM runtime-base AS luanti-clients

RUN apt-get update && \
    apt-get install --yes --no-install-recommends \
    fonts-dejavu-core \
    libegl-mesa0 \
    libegl1 \
    libfreetype6 \
    libgl1 \
    libgl1-mesa-dri \
    libglx-mesa0 \
    libjpeg62-turbo \
    libogg0 \
    libopenal1 \
    libopengl0 \
    libpng16-16t64 \
    libsdl2-2.0-0 \
    libvorbis0a \
    libvorbisfile3 && \
    rm -rf /var/lib/apt/lists/*

COPY --from=luanti-5.16.1-client /work/dist/luanti-5.16.1-client /work/dist/luanti-5.16.1-client
COPY --from=luanti-5.15.2-client /work/dist/luanti-5.15.2-client /work/dist/luanti-5.15.2-client
COPY --from=luanti-5.14.0-client /work/dist/luanti-5.14.0-client /work/dist/luanti-5.14.0-client


FROM runtime-base AS luanti-servers

COPY --from=luanti-5.16.1-server /work/dist/luanti-5.16.1-server /work/dist/luanti-5.16.1-server
COPY --from=luanti-5.15.2-server /work/dist/luanti-5.15.2-server /work/dist/luanti-5.15.2-server
COPY --from=luanti-5.14.0-server /work/dist/luanti-5.14.0-server /work/dist/luanti-5.14.0-server

COPY --from=luanti-5.16.1-server /work/dist/luanti-5.16.1-server/minetest.conf.example /etc/minetest/minetest.conf

EXPOSE 30000/udp 30000/tcp
