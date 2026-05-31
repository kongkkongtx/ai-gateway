FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/ai-gateway ./cmd/gateway

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/ai-gateway .
COPY configs/ ./configs/

EXPOSE 8080

ENTRYPOINT ["./ai-gateway"]
CMD ["-config", "configs/gateway.yaml"]