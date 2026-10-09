FROM golang:1.27.1-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY proto ./proto
COPY internals ./internals
COPY go-deployment-2 ./go-deployment-2
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/grpc-server ./go-deployment-2/grpc-server \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/rabbit-writer ./go-deployment-2/rabbit-writer

FROM alpine:3.24
RUN apk --no-cache add ca-certificates
COPY --from=builder /out/grpc-server /app/grpc-server
COPY --from=builder /out/rabbit-writer /app/rabbit-writer
USER 10001:10001
EXPOSE 50051
CMD ["/app/grpc-server"]
