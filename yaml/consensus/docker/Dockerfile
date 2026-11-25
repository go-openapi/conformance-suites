# SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
# SPDX-License-Identifier: Apache-2.0
#
# The consensus image: every YAML implementation the consensus runner consults, and the runner.
#
# Build it from the repository root:
#
#   docker build -f yaml/consensus/docker/Dockerfile -t conformance-consensus .
#
# To score a local checkout of go-openapi/go-yaml instead of the pinned version, give it as a build context:
#
#   docker build -f yaml/consensus/docker/Dockerfile --build-context goyaml=../go-yaml -t conformance-consensus .
#
# Run it on a corpus file in the current directory, as your own user so that the cache and the report are yours:
#
#   docker run --rm conformance-consensus check
#   docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/work" conformance-consensus \
#     run -cache /work/.consensus-cache -corpus-ref "$(git rev-parse --short HEAD)" /work/yaml-compact.jsonl.gz > results.jsonl
#   docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/work" conformance-consensus \
#     report -o /work/report /work/results.jsonl
#
# Every version is pinned below. Each was the latest release on 2026-11-24:
#
#   Debian                     trixie (stable), image trixie-20261123-slim
#   Go                         1.27.4
#   yaml-reference-parser      main d58347b (2026-06-14), ext-perl ff50c26 (2021-12-28)
#   libfyaml                   1.0.0-rc2 (2026-11-11): the C sources for fy-tool, the PyPI binding 1.0.0rc2
#   YAMLStar                   v0.1.28 (2026-10-28), linux-x64 release
#   go.yaml.in/yaml/v3         v3.0.6, compiled into the runner
#   go-openapi/go-yaml         v0.0.0-20261123092944-569e0d675ee1 (origin/master), unless a goyaml context is given

ARG DEBIAN=debian:trixie-20261123-slim
ARG GOLANG=golang:1.27.4-trixie

# The go-yaml checkout to score. Empty unless the build is given --build-context goyaml=<dir>.
FROM scratch AS goyaml

# ---------------------------------------------------------------------------------------------------------------------
FROM ${GOLANG} AS build

WORKDIR /src/conformance-suites
COPY . .

# The runner, with go.yaml.in/yaml/v3 compiled in. yaml/consensus requires the root module through a replace.
RUN cd yaml/consensus && GOWORK=off CGO_ENABLED=0 go build -trimpath -o /out/consensus ./cmd/consensus

# goyaml-json, built from the pinned go-openapi/go-yaml or from the goyaml context.
COPY --from=goyaml / /src/go-yaml/
RUN cd yaml/consensus/docker/goyaml-json \
 && if [ -f /src/go-yaml/go.mod ]; then \
      GOWORK=off go mod edit -replace github.com/go-openapi/go-yaml=/src/go-yaml; \
      echo "go-openapi/go-yaml: local checkout $(cd /src/go-yaml && git rev-parse --short HEAD 2>/dev/null || echo 'of unknown revision')" > /out/goyaml-json.version; \
    else \
      GOWORK=off go list -m -f 'go-openapi/go-yaml: {{.Version}}' github.com/go-openapi/go-yaml > /out/goyaml-json.version; \
    fi \
 && GOWORK=off GOFLAGS=-mod=mod CGO_ENABLED=0 go build -trimpath -o /out/goyaml-json .

# ---------------------------------------------------------------------------------------------------------------------
FROM ${DEBIAN} AS libfyaml

ARG LIBFYAML=1.0.0-rc2
ARG LIBFYAML_SHA256=8a88775eef05eaeea865f80b4a2fd5703bfa87ae46a3cc4654fe7095c5a1f6b3

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl build-essential pkg-config m4 \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /src
# _GNU_SOURCE: libfyaml-align.h calls posix_memalign without it, and trixie's GCC 14 refuses an implicit declaration.
ENV CFLAGS="-O2 -D_GNU_SOURCE"
RUN curl -fsSL -o libfyaml.tar.gz "https://github.com/pantoniou/libfyaml/releases/download/v${LIBFYAML}/libfyaml-${LIBFYAML}.tar.gz" \
 && echo "${LIBFYAML_SHA256}  libfyaml.tar.gz" | sha256sum -c - \
 && tar -xzf libfyaml.tar.gz \
 && cd "libfyaml-${LIBFYAML}" \
 && ./configure --prefix=/opt/libfyaml --disable-shared --enable-static \
 && make -j"$(nproc)" \
 && make install

# ---------------------------------------------------------------------------------------------------------------------
FROM ${DEBIAN}

ARG REFPARSER_MAIN=d58347b997eea4d7e5ba33b36871535dad6db081
ARG REFPARSER_EXT=ff50c26f21c38b6eb99e1c5f1ddbb97fe1461349
ARG LIBFYAML_BINDING=1.0.0rc2
ARG YAMLSTAR_VERSION=0.1.28
ARG YAMLSTAR_SHA256=97562b66084df62d0339dbc34500212077fa25601c349fd61261d9c482859404

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl perl python3 python3-pip xz-utils \
 && rm -rf /var/lib/apt/lists/*

# The scripts the host uses, so the image and a host install the same thing.
COPY hack/conformance/install-reference-parser.sh hack/conformance/install-libfyaml.sh /opt/install/

# HOME is writable by any user the image runs as: libfyaml's binding keeps a cache under it.
ENV HOME=/tmp \
    REFPARSER_HOME=/opt/yaml-reference-parser \
    LIBFYAML_HOME=/opt/libfyaml-python \
    FY_TOOL=/usr/local/bin/fy-tool \
    YAMLSTAR=/usr/local/bin/yaml \
    GOYAML_JSON=/usr/local/bin/goyaml-json

RUN REFPARSER_MAIN=${REFPARSER_MAIN} REFPARSER_EXT=${REFPARSER_EXT} /opt/install/install-reference-parser.sh

RUN PIP_BREAK_SYSTEM_PACKAGES=1 LIBFYAML_VERSION=${LIBFYAML_BINDING} /opt/install/install-libfyaml.sh

RUN curl -fsSL -o /tmp/yamlstar.tar.xz "https://github.com/yaml/yamlstar/releases/download/v${YAMLSTAR_VERSION}/yamlstar-${YAMLSTAR_VERSION}-linux-x64.tar.xz" \
 && echo "${YAMLSTAR_SHA256}  /tmp/yamlstar.tar.xz" | sha256sum -c - \
 && tar -xJf /tmp/yamlstar.tar.xz -C /tmp \
 && install -m 0755 "/tmp/yamlstar-${YAMLSTAR_VERSION}-linux-x64/yaml" /usr/local/bin/yaml \
 && rm -rf /tmp/yamlstar*

COPY --from=libfyaml /opt/libfyaml/bin/fy-tool /usr/local/bin/fy-tool
COPY --from=build /out/consensus /out/goyaml-json /out/goyaml-json.version /usr/local/bin/

WORKDIR /work
ENTRYPOINT ["consensus"]
CMD ["check"]
