# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM node:24-bookworm-slim AS frontend
WORKDIR /src

COPY frontend/package*.json ./frontend/
RUN --mount=type=cache,target=/root/.npm \
    cd frontend && if [ -f package-lock.json ]; then npm ci; else npm install; fi

COPY frontend ./frontend
RUN mkdir -p core/admin && cd frontend && npm run build

FROM --platform=$BUILDPLATFORM golang:1.26.5-bookworm AS builder
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
COPY --from=frontend /src/core/admin ./core/admin

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build \
    -trimpath \
    -ldflags="-s -w -X github.com/qninq/sillyGirlPro/core.compiled_at=${VERSION}" \
    -o /out/sillyGirl .

FROM --platform=$TARGETPLATFORM python:3.12-slim-bookworm AS python-runtime

FROM --platform=$TARGETPLATFORM node:24-bookworm-slim
WORKDIR /app

COPY --from=python-runtime /usr/local /usr/local

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && corepack enable \
    && corepack prepare pnpm@11.16.0 --activate \
    && mkdir -p /app/node-runtime \
    && cd /app/node-runtime \
    && printf '{"name":"sillygirl-node-runtime","private":true,"version":"1.0.0"}\n' > package.json \
    && printf 'packages:\n  - .\nallowBuilds:\n  protobufjs: true\n' > pnpm-workspace.yaml \
    && pnpm --allow-build=protobufjs add @grpc/grpc-js@^1.8.18 google-protobuf@^3.21.2 \
    && mkdir -p /data/plugins /data/conf \
    && python3.12 -m pip install --no-cache-dir pipx \
    && mkdir -p /tmp/sillygirl-python-runtime/sillygirl_python_runtime \
    && printf '[project]\nname = "sillygirl-python-runtime"\nversion = "1.0.0"\nrequires-python = ">=3.12"\n\n[project.scripts]\nsillygirl-python-runtime = "sillygirl_python_runtime:main"\n' > /tmp/sillygirl-python-runtime/pyproject.toml \
    && printf 'def main():\n    return 0\n' > /tmp/sillygirl-python-runtime/sillygirl_python_runtime/__init__.py \
    && PIPX_HOME=/data/plugins/python_packages PIPX_BIN_DIR=/data/plugins/python_packages/bin python3.12 -m pipx install --force --python python3.12 /tmp/sillygirl-python-runtime \
    && PIPX_HOME=/data/plugins/python_packages PIPX_BIN_DIR=/data/plugins/python_packages/bin python3.12 -m pipx runpip sillygirl-python-runtime install --upgrade --no-cache-dir "grpcio==1.83.0" "protobuf==7.35.1" \
    && rm -rf /tmp/sillygirl-python-runtime \
    && ln -s /data/plugins /app/plugins \
    && ln -s /data/conf /app/conf

COPY --from=builder /out/sillyGirl /app/sillyGirl
COPY --from=builder /src/proto3/sillygirl.js /app/proto3/sillygirl.js
COPY --from=builder /src/proto3/sillygirl.d.ts /app/proto3/sillygirl.d.ts
COPY --from=builder /src/proto3/sillygirl.py /app/proto3/sillygirl.py
COPY --from=builder /src/proto3/srpc.js /app/proto3/srpc.js
COPY --from=builder /src/proto3/srpc_pb2.py /app/proto3/srpc_pb2.py
COPY --from=builder /src/proto3/srpc_pb2_grpc.py /app/proto3/srpc_pb2_grpc.py

ENV TZ=Asia/Shanghai \
    SILLYGIRL_DATA_PATH=/data \
    SILLYGIRL_NODE_PATH=/app/node-runtime/node_modules \
    SILLYGIRL_PYTHON_BIN=python3.12 \
    SILLYGIRL_PYTHON_PATH=/app/proto3 \
    PIPX_HOME=/data/plugins/python_packages \
    PIPX_BIN_DIR=/data/plugins/python_packages/bin \
    NODE_PATH=/app/node-runtime/node_modules \
    PYTHONPATH=/app/proto3
EXPOSE 8080 50051
VOLUME ["/data"]

ENTRYPOINT ["sh", "-c", "mkdir -p /data/plugins /data/conf; exec /app/sillyGirl \"$@\"", "--"]
