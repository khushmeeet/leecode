FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /leecode .

FROM alpine:3.21
# ca-certificates: Litestream needs TLS to reach S3/R2.
RUN apk add --no-cache ca-certificates
# Statically-linked Litestream binary pulled from the official image.
COPY --from=litestream/litestream:0.5.12 /usr/local/bin/litestream /usr/local/bin/litestream
COPY --from=build /leecode /leecode
COPY litestream.yml /etc/litestream.yml
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENV DB_PATH=/data/interview.db
EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
