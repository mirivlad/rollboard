FROM node:20-alpine AS frontend-build
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
# English is imported from the repository-root catalogs as the bundled
# fallback, so this stage needs them too. The path mirrors the repository
# layout: WORKDIR is /src/frontend, so the root is /src.
COPY locales/ /src/locales/
RUN npm run build

FROM golang:1.24-alpine AS backend-build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/rollboard ./cmd/server

FROM alpine:3.21
RUN addgroup -S rollboard && adduser -S -G rollboard -u 10001 rollboard
WORKDIR /app
COPY --from=backend-build /out/rollboard /app/rollboard
COPY --from=frontend-build /src/frontend/dist /app/frontend
# Translation catalogs are read from disk at request time, not compiled into
# the bundle, so mounting a volume over /app/locales adds or corrects a
# language without rebuilding this image.
COPY locales/ /app/locales/
USER rollboard
EXPOSE 8080
ENV ROLLBOARD_ADDR=0.0.0.0:8080 \
    ROLLBOARD_STATIC_DIR=/app/frontend \
    ROLLBOARD_LOCALES_DIR=/app/locales
ENTRYPOINT ["/app/rollboard"]
