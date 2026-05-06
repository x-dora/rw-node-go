# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build

ARG PROJECT_VERSION=1.0.0
ARG NODE_VERSION=2.7.0
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w -X github.com/x-dora/rw-node-go/internal/version.ProjectVersion=${PROJECT_VERSION} -X github.com/x-dora/rw-node-go/internal/version.NodeVersion=${NODE_VERSION} -X github.com/x-dora/rw-node-go/internal/version.Commit=${COMMIT} -X github.com/x-dora/rw-node-go/internal/version.BuildDate=${BUILD_DATE}" \
    -o /out/rw-node-go ./cmd/rw-node-go

FROM alpine:3.23

RUN addgroup -S rw-node && adduser -S -G rw-node rw-node \
    && mkdir -p /opt/rw-node-go/xray /usr/local/share/xray \
    && chown -R rw-node:rw-node /opt/rw-node-go

COPY --from=build /out/rw-node-go /usr/local/bin/rw-node-go
COPY docker/entrypoint.sh /usr/local/bin/rw-node-go-entrypoint

RUN chmod +x /usr/local/bin/rw-node-go /usr/local/bin/rw-node-go-entrypoint

ENV REQUIRE_SECRET_KEY=true

USER rw-node
EXPOSE 2222
ENTRYPOINT ["/usr/local/bin/rw-node-go-entrypoint"]
