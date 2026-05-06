# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build

ARG PROJECT_VERSION=1.0.0
ARG NODE_VERSION=2.7.0
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -ldflags "-s -w -X github.com/x-dora/rw-node-go/internal/version.ProjectVersion=${PROJECT_VERSION} -X github.com/x-dora/rw-node-go/internal/version.NodeVersion=${NODE_VERSION} -X github.com/x-dora/rw-node-go/internal/version.Commit=${COMMIT} -X github.com/x-dora/rw-node-go/internal/version.BuildDate=${BUILD_DATE}" \
    -o /out/rw-node-go ./cmd/rw-node-go

FROM --platform=$BUILDPLATFORM alpine:3.23 AS runtime-files

RUN addgroup -S -g 10001 rw-node \
    && adduser -S -D -H -u 10001 -G rw-node rw-node \
    && mkdir -p /opt/rw-node-go/xray /usr/local/share/xray \
    && chown -R 10001:10001 /opt/rw-node-go /usr/local/share/xray

FROM alpine:3.23

COPY --from=runtime-files /etc/passwd /etc/passwd
COPY --from=runtime-files /etc/group /etc/group
COPY --from=runtime-files --chown=10001:10001 /opt/rw-node-go /opt/rw-node-go
COPY --from=runtime-files --chown=10001:10001 /usr/local/share/xray /usr/local/share/xray
COPY --from=build --chmod=755 /out/rw-node-go /usr/local/bin/rw-node-go
COPY --chmod=755 docker/entrypoint.sh /usr/local/bin/rw-node-go-entrypoint

ENV REQUIRE_SECRET_KEY=true

USER 10001:10001
EXPOSE 2222
ENTRYPOINT ["/usr/local/bin/rw-node-go-entrypoint"]
