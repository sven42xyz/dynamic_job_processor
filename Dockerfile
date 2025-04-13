FROM alpine:latest

WORKDIR /app
COPY wavely /app/wavely

VOLUME ["/app/cache", "/app/logs", "/app/config"]

EXPOSE 4224
ENTRYPOINT ["/app/wavely"]
