# syntax=docker/dockerfile:1
# check=error=true
# SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
# SPDX-License-Identifier: LGPL-3.0-or-later

ARG ALPINE_VERSION=3.24
ARG BUILD_JOBS=2

FROM alpine:${ALPINE_VERSION} AS build-base

ARG BUILD_JOBS
ARG LUAJIT_REF=v2.1

ENV CMAKE_BUILD_PARALLEL_LEVEL=${BUILD_JOBS}

RUN apk add --no-cache \
    build-base \
    cmake \
    curl-dev \
    freetype-dev \
    gettext-dev \
    git \
    gmp-dev \
    hiredis-dev \
    jpeg-dev \
    jsoncpp-dev \
    leveldb-dev \
    libogg-dev \
    libpng-dev \
    libspatialindex-dev \
    libvorbis-dev \
    mesa-dev \
    ninja \
    openal-soft-dev \
    postgresql-dev \
    sdl2-dev \
    sqlite-dev \
    zlib-dev \
    zstd-dev

ADD --keep-git-dir https://luajit.org/git/luajit.git?branch=${LUAJIT_REF} /usr/src/luajit

RUN make -C /usr/src/luajit amalg && \
    make -C /usr/src/luajit install


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


FROM alpine:${ALPINE_VERSION} AS runtime

RUN apk add --no-cache \
    curl \
    freetype \
    gmp \
    hiredis \
    jsoncpp \
    leveldb \
    libgcc \
    libintl \
    libjpeg-turbo \
    libogg \
    libpng \
    libpq \
    libspatialindex \
    libstdc++ \
    libvorbis \
    mesa-gl \
    openal-soft \
    postgresql \
    sdl2 \
    sqlite-libs \
    zlib \
    zstd-libs && \
    adduser -D -u 30000 -h /var/lib/minetest minetest && \
    mkdir -p \
    /etc/minetest \
    /var/lib/minetest/games \
    /var/lib/minetest/mods \
    /var/lib/minetest/worlds && \
    chown -R minetest:minetest /etc/minetest /var/lib/minetest

COPY --from=build-base /usr/local/lib/libluajit* /usr/local/lib/

COPY --chown=minetest:minetest --from=luanti-5.16.1-client /work/dist/luanti-5.16.1-client /work/dist/luanti-5.16.1-client
COPY --chown=minetest:minetest --from=luanti-5.16.1-server /work/dist/luanti-5.16.1-server /work/dist/luanti-5.16.1-server
COPY --chown=minetest:minetest --from=luanti-5.15.2-client /work/dist/luanti-5.15.2-client /work/dist/luanti-5.15.2-client
COPY --chown=minetest:minetest --from=luanti-5.15.2-server /work/dist/luanti-5.15.2-server /work/dist/luanti-5.15.2-server
COPY --chown=minetest:minetest --from=luanti-5.14.0-client /work/dist/luanti-5.14.0-client /work/dist/luanti-5.14.0-client
COPY --chown=minetest:minetest --from=luanti-5.14.0-server /work/dist/luanti-5.14.0-server /work/dist/luanti-5.14.0-server

COPY --chown=minetest:minetest --from=luanti-5.16.1-server /work/dist/luanti-5.16.1-server/minetest.conf.example /etc/minetest/minetest.conf

ENV LUANTI_GAME_PATH=/var/lib/minetest/games \
    LUANTI_MOD_PATH=/var/lib/minetest/mods \
    LUANTI_WORLD_PATH=/var/lib/minetest/worlds \
    MINETEST_GAME_PATH=/var/lib/minetest/games \
    MINETEST_MOD_PATH=/var/lib/minetest/mods \
    MINETEST_WORLD_PATH=/var/lib/minetest/worlds

WORKDIR /var/lib/minetest

USER minetest:minetest

EXPOSE 30000/udp 30000/tcp

VOLUME ["/var/lib/minetest", "/etc/minetest"]

ENTRYPOINT ["/work/dist/luanti-5.16.1-server/bin/luantiserver"]
CMD ["--config", "/etc/minetest/minetest.conf", "--world", "/var/lib/minetest/worlds/world", "--logfile", "/var/lib/minetest/debug.txt"]
