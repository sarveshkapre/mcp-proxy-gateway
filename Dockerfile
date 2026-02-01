FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 go build -o /bin/mcp-proxy-gateway ./cmd/mcp-proxy-gateway

FROM alpine:3.20
RUN adduser -D -H -s /sbin/nologin appuser
USER appuser
COPY --from=builder /bin/mcp-proxy-gateway /usr/local/bin/mcp-proxy-gateway
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/mcp-proxy-gateway"]
