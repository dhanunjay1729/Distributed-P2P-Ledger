# ---------- BUILD STAGE ----------
FROM golang:1.22.12-alpine AS builder

WORKDIR /app

# Copy go modules first (for caching)
COPY go.mod ./
RUN go mod download

# Copy full project
COPY . .

# Build binary
RUN go build -o node ./cmd/node


# ---------- RUN STAGE ----------
FROM alpine:latest

WORKDIR /root/

# Copy built binary
COPY --from=builder /app/node .

# Expose internal port
EXPOSE 8000

# Run binary
CMD ["./node"]