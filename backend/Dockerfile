# ── Build stage ──────────────────────────────────────────
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /app/server .

# ── Runtime stage ────────────────────────────────────────
FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 1000 appuser

WORKDIR /app
COPY --from=builder /app/server .

USER appuser

EXPOSE 8080

ENV PORT=8080

ENTRYPOINT ["./server"]

