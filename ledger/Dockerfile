FROM golang:1.25-alpine AS builder
WORKDIR /src/ledger

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/ledger ./cmd

FROM alpine:3.23 AS runner
WORKDIR /app
COPY --from=builder /out/ledger ./ledger
ENTRYPOINT ["./ledger"]
