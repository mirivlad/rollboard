FROM node:20-alpine AS frontend-build
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
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
USER rollboard
EXPOSE 8080
ENV ROLLBOARD_ADDR=0.0.0.0:8080 \
    ROLLBOARD_STATIC_DIR=/app/frontend
ENTRYPOINT ["/app/rollboard"]
