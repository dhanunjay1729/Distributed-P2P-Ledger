# ---------- BUILD STAGE ----------
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# Copy full project
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/node ./node_a


# ---------- RUN STAGE ----------
FROM alpine:3.20
WORKDIR /app

COPY --from=builder /bin/node /app/node

EXPOSE 8080
ENTRYPOINT ["/app/node"]