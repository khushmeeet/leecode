FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /leecode .

FROM alpine:3.21
COPY --from=build /leecode /leecode
ENV DB_PATH=/data/interview.db
EXPOSE 8080
CMD ["/leecode"]
