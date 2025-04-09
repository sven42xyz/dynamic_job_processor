# Build-Stage mit Go 1.24
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Dependencies laden
COPY go.mod go.sum ./
RUN go mod download

# Quellcode kopieren
COPY . .

# Statisches Binary bauen
RUN CGO_ENABLED=0 go build -o wavely ./cmd/main.go

# Finales Image
FROM alpine:latest

WORKDIR /app

# Binary aus Builder übernehmen
COPY --from=builder /app/wavely /app/wavely

# Konfigurationsordner & Logs
COPY config /app/config
VOLUME ["/app/cache"]
VOLUME ["/app/logs"]

EXPOSE 4224

ENTRYPOINT ["/app/wavely"]
