# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/pagechat ./cmd/pagechat

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/pagechat /usr/local/bin/pagechat

EXPOSE 8080

ENTRYPOINT ["pagechat"]
CMD ["--port", "8080", "--filter"]
