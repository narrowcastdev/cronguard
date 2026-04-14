FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /cronguard ./cmd/cronguard

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && mkdir -p /data
COPY --from=builder /cronguard /usr/local/bin/cronguard
EXPOSE 8099
ENTRYPOINT ["cronguard", "--data", "/data/cronguard.db"]
