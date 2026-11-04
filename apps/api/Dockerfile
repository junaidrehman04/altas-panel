FROM golang:1.22.2-alpine AS dev
WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV ALTAS_ADDR=:8080
EXPOSE 8080
CMD ["go", "run", "./apps/api/cmd/server"]

FROM golang:1.22.2-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
COPY apps ./apps
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/altas-api ./apps/api/cmd/server

FROM alpine:3.21 AS runtime
RUN apk add --no-cache ca-certificates su-exec \
    && addgroup -g 65532 -S altas \
    && adduser -S -D -H -u 65532 -G altas altas \
    && mkdir -p /data \
    && chown altas:altas /data
COPY --from=build /out/altas-api /usr/local/bin/altas-api
COPY apps/api/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod 755 /usr/local/bin/docker-entrypoint.sh /usr/local/bin/altas-api
EXPOSE 8080
VOLUME /data
ENV ALTAS_ADDR=:8080 \
    ALTAS_DB=/data/altas.db
ENTRYPOINT ["docker-entrypoint.sh"]
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=5 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/api/health || exit 1
