# Both build stages are pinned to the machine doing the building, never the
# target architecture. The frontend bundle is architecture-independent, and Go
# cross-compiles, so an arm64 image is produced without emulating arm64 — which
# takes minutes rather than the tens of minutes QEMU needs to run a compiler.
FROM --platform=$BUILDPLATFORM node:20-alpine AS frontend-build
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
# English is imported from the repository-root catalogs as the bundled
# fallback, so this stage needs them too. The path mirrors the repository
# layout: WORKDIR is /src/frontend, so the root is /src.
COPY locales/ /src/locales/
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS backend-build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# TARGETARCH is supplied by buildx for each platform being produced.
ARG TARGETARCH
ARG TARGETOS
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags='-s -w' -o /out/rollboard ./cmd/server

FROM alpine:3.21
# Nothing is executed in this stage, only copied, so producing an arm64 image
# needs no emulation at all. Running adduser here would have been the one
# command forcing QEMU into the build.
WORKDIR /app
COPY --from=backend-build /out/rollboard /app/rollboard
COPY --from=frontend-build /src/frontend/dist /app/frontend
# Translation catalogs are read from disk at request time, not compiled into
# the bundle, so mounting a volume over /app/locales adds or corrects a
# language without rebuilding this image.
COPY locales/ /app/locales/
# A numeric UID rather than a named user: the server is a static binary and
# never looks itself up in /etc/passwd.
USER 10001:10001
EXPOSE 8080
ENV ROLLBOARD_ADDR=0.0.0.0:8080 \
    ROLLBOARD_STATIC_DIR=/app/frontend \
    ROLLBOARD_LOCALES_DIR=/app/locales
ENTRYPOINT ["/app/rollboard"]
