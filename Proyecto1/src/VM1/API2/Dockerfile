FROM golang:1.26.5-alpine AS builder

WORKDIR /app/api2

COPY . .
RUN go build -o main .

FROM alpine
WORKDIR /root/
COPY  --from=builder /app/api2/main .
EXPOSE 8081
CMD ["./main"]