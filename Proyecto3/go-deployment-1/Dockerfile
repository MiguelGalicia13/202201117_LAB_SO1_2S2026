FROM golang:1.27.1-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download


COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/rest-receiver ./go-deployment-1/rest-receiver
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/grpc-client ./go-deployment-1/grpc-client


FROM alpine:3.19
WORKDIR /app
RUN apk --no-cache add ca-certificates

COPY --from=builder /app/bin/rest-receiver /app/rest-receiver
COPY --from=builder /app/bin/grpc-client /app/grpc-client

EXPOSE 8080 8081
CMD ["/app/rest-receiver"]