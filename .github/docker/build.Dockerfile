FROM ubuntu:24.04

# Reuse this layer locally, refreshing OS packages at least once a month.
ARG CACHE_EPOCH
RUN echo "Dependency refresh: ${CACHE_EPOCH}" \
    && apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates git curl python3 unzip \
       gcc g++ gcc-mingw-w64-x86-64 g++-mingw-w64-x86-64 openssh-client \
       libwebkit2gtk-4.1-0 xvfb xauth dbus-x11 libgl1-mesa-dri libglx-mesa0 \
       libegl-mesa0 fonts-dejavu-core fonts-noto-cjk lrzsz \
    && rm -rf /var/lib/apt/lists/*

ENV PATH="/opt/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin" \
    CGO_ENABLED=0 \
    GOTOOLCHAIN=local \
    GOFLAGS=-mod=readonly \
    GOCACHE=/cache/go-build \
    GOMODCACHE=/cache/go-mod \
    XDG_CACHE_HOME=/cache/native
WORKDIR /workspace
