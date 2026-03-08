# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Download dependencies first (cached layer)
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Build — CGO_ENABLED=0 because we use modernc.org/sqlite (pure Go)
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -trimpath \
    -o contacthub \
    ./cmd/contacthub

# ---- Final stage ----
# distroless/static includes CA certs and timezone data, nothing else
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /build/contacthub .

# Data directory for SQLite DB
VOLUME ["/data"]

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/app/contacthub"]
