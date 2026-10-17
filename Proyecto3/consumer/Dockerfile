FROM golang:1.27.1-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY consumer ./consumer
COPY internals ./internals
RUN CGO_ENABLED=0 go build -o /out/consumer ./consumer

FROM alpine:3.24
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /out/consumer /app/consumer
USER 10001:10001
CMD ["/app/consumer"]

