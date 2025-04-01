# Verwenden Sie ein offizielles Go-Image als Basis
FROM golang:1.24-alpine AS builder

# Setzen Sie das Arbeitsverzeichnis im Container
WORKDIR /app

# Kopieren Sie die go.mod und go.sum Dateien, um Abhängigkeiten zu verwalten
COPY go.mod go.sum ./

# Laden Sie die Abhängigkeiten herunter
RUN go mod download

# Kopieren Sie den Quellcode
COPY . .

# Bauen Sie die Go-Anwendung
RUN go build -o microservice

# Erstellen Sie ein schlankes Laufzeit-Image
FROM alpine:latest

# Installieren Sie ca-certificates für HTTPS-Unterstützung
RUN apk --no-cache add ca-certificates

# Setzen Sie das Arbeitsverzeichnis
WORKDIR /app

# Kopieren Sie die erstellte ausführbare Datei aus dem Builder-Image
COPY --from=builder /app/microservice .

# Kopieren Sie die optionale Konfigurationsdatei
COPY config.yaml .

# Machen Sie die ausführbare Datei ausführbar
RUN chmod +x microservice

# Definieren Sie den Befehl zum Starten des Microservice
CMD ["./microservice"]

# Stellen Sie den Port bereit, auf dem die Anwendung läuft
EXPOSE 8080