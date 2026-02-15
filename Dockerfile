# Build Stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod ./
# COPY go.sum ./

RUN go mod download

COPY . .

# Run tests
RUN go test -v ./...

RUN CGO_ENABLED=0 GOOS=linux go build -o traefik-auth-code-middleware .

# Final Stage
FROM gcr.io/distroless/static-debian12

COPY --from=builder /app/traefik-auth-code-middleware /usr/local/bin/traefik-auth-code-middleware

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/traefik-auth-code-middleware"]
