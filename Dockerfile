# Multi-stage build: compile Go binary, then copy into a minimal runtime image.
FROM golang:1.24-bookworm AS builder
WORKDIR /app

# Pre-fetch dependencies for better build caching.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source and build the server.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/tether ./cmd

# Minimal runtime image.
FROM gcr.io/distroless/base-debian12
WORKDIR /
COPY --from=builder /bin/tether /bin/tether

# Default listen port (matches PORT env default of 8080).
EXPOSE 8080

# The server reads configuration from environment (DISCORD_TOKEN, GUILD_ID, etc.).
ENTRYPOINT ["/bin/tether"]
