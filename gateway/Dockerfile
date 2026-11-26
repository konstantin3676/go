FROM golang:1.25-alpine AS builder
WORKDIR /src/gateway

COPY go.mod go.sum ./
COPY --from=ledger_src go.mod go.sum /src/ledger/
RUN go mod download

COPY . .
COPY --from=ledger_src . /src/ledger/
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/gateway .

FROM alpine:3.23 AS runner
WORKDIR /app
COPY --from=builder /out/gateway ./gateway
EXPOSE 8080
ENTRYPOINT ["./gateway"]
