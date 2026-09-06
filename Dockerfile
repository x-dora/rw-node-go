# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26.5-alpine AS build

ARG COMMIT=unknown
ARG BUILD_DATE=unknown
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    COMMIT=${COMMIT} BUILD_DATE=${BUILD_DATE} \
    go run ./cmd/rw-build -repo-root /src -o /out/rw-node-go -goos $TARGETOS -goarch $TARGETARCH

# Download geodat into a staging directory, matching Xray-core Docker assets.
ADD https://raw.githubusercontent.com/Loyalsoldier/v2ray-rules-dat/release/geoip.dat /tmp/geodat/geoip.dat
ADD https://raw.githubusercontent.com/Loyalsoldier/v2ray-rules-dat/release/geosite.dat /tmp/geodat/geosite.dat

# geocheck binary for the stats/get-geocheck route, matching the official node image.
FROM --platform=$BUILDPLATFORM alpine:3.23 AS geocheck

ARG GEOCHECK_VERSION=0.3.0
ARG GEOCHECK_RELEASE_URL=https://github.com/remnawave/geocheck/releases/download
ARG TARGETARCH

RUN apk add --no-cache curl \
    && cd /tmp \
    && ARCHIVE="geocheck_linux_${TARGETARCH}.tar.gz" \
    && curl -fsSL -O "${GEOCHECK_RELEASE_URL}/v${GEOCHECK_VERSION}/${ARCHIVE}" \
    && curl -fsSL -O "${GEOCHECK_RELEASE_URL}/v${GEOCHECK_VERSION}/checksums.txt" \
    && grep "  ${ARCHIVE}\$" checksums.txt | sha256sum -c - \
    && tar -xzf "${ARCHIVE}" geocheck \
    && install -m 0755 geocheck /usr/local/bin/geocheck \
    && rm -rf /tmp/*

FROM --platform=$BUILDPLATFORM alpine:3.23 AS runtime-files

RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 rw-node \
    && adduser -S -D -H -u 10001 -G rw-node rw-node \
    && mkdir -p /opt/rw-node-go/xray /usr/local/share/xray /tmp \
    && chmod 1777 /tmp \
    && chown -R 10001:10001 /opt/rw-node-go /usr/local/share/xray

FROM scratch

COPY --from=runtime-files /etc/passwd /etc/passwd
COPY --from=runtime-files /etc/group /etc/group
COPY --from=runtime-files /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=runtime-files /tmp /tmp
COPY --from=runtime-files --chown=10001:10001 /opt/rw-node-go /opt/rw-node-go
COPY --from=runtime-files --chown=10001:10001 /usr/local/share/xray /usr/local/share/xray
COPY --from=build --chown=10001:10001 --chmod=644 /tmp/geodat/*.dat /usr/local/share/xray/
COPY --from=geocheck --chmod=755 /usr/local/bin/geocheck /usr/local/bin/geocheck
COPY --from=build --chmod=755 /out/rw-node-go /usr/local/bin/rw-node-go

ENV REQUIRE_SECRET_KEY=true
ENV LOG_COLOR=always
ENV XRAY_LOCATION_ASSET=/usr/local/share/xray

USER 0:0
EXPOSE 2222
ENTRYPOINT ["/usr/local/bin/rw-node-go"]
