FROM golang:1.26.5-alpine AS builder

WORKDIR /app/api1

COPY . .
RUN go build -o main .

FROM alpine
WORKDIR /root/
COPY  --from=builder /app/api1/main .
EXPOSE 8080
CMD ["./main"]