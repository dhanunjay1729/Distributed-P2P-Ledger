FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/node ./cmd/node \
    && mkdir -p /out/data

FROM scratch
WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/node /app/node
COPY --from=builder --chown=10001:10001 /out/data /app/data
COPY --from=builder /src/config/peers /app/config/peers
USER 10001:10001
ENV PORT=8080 BIND_ADDR=0.0.0.0 CHAIN_FILE=/app/data/chain.json
EXPOSE 8080
HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=6 CMD ["/app/node", "-healthcheck"]
ENTRYPOINT ["/app/node"]
