package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"time"
)

type rabbitSession struct {
	connection *amqp.Connection
	channel    *amqp.Channel
	socket     net.Conn
	returns    <-chan amqp.Return
}



func run() error {
	url:= os.Getenv("RABBITMQ_URL")
	if url == "" {
		log.Println("[ERROR] RABBITMQ_URL not set")
		return fmt.Errorf("RABBITMQ_URL not set")
	}
	queueName := os.Getenv("RABBITMQ_QUEUE")
	if queueName == "" {
		log.Printf("[ERROR] %s not set", queueName)
		return fmt.Errorf("%s not set", queueName)
	}
	valkey_url := os.Getenv("VALKEY_URL")
	if valkey_url == "" {
		log.Println("[ERROR] VALKEY_URL not set")
		return fmt.Errorf("VALKEY_URL not set")
	}
	opts,err:= redis.ParseURL(valkey_url)
	if err != nil{
		log.Println("[ERROR] INVALID URL")
		return fmt.Errorf("invalid url")
	}
	var rdb = redis.NewClient(opts)
	
	defer rdb.Close()
	rabbitCtx, cancelRabbit := context.WithCancel(context.Background())
	defer cancelRabbit()
	valkeyCtx, cancelValkey := context.WithCancel(context.Background())
	defer cancelValkey()
	session, err := connect(rabbitCtx, url, queueName)
	if err != nil{
		log.Printf("[ERROR] COULD NOT CONNECT: %s ", err)
		return err
	}
	defer session.connection.Close()
	res := rdb.Ping(valkeyCtx)
	if res.Err() != nil {
		log.Printf("[ERROR] COULD NOT PING: %s", res.Err())
		return res.Err()
	}
	err = session.channel.Qos(1, 0, false)
	if err != nil {
		log.Printf("[ERROR] COULD NOT SET QOS: %s", err)
		return err
	}

	deliveries, err := session.channel.ConsumeWithContext(rabbitCtx, queueName, "", false, false, false, false, nil)
	if err != nil {
		log.Printf("[ERROR] COULD NOT CONSUME: %s", err)
		return err
	}
	for{
		select {
			case <- rabbitCtx.Done():
				log.Println("[ERROR] RABBITMQ CONTEXT DONE")
				return nil
			case deliveriy, ok := <- deliveries:
				if !ok{
					log.Println("[ERROR] DELIVERIES CHANNEL CLOSED")
					return fmt.Errorf("DELIVERIES CHANNEL CLOSED")
				}
				log.Printf("[LOG] MESSAGE RECIEVED: %s", deliveriy.Body)
				/*
				 * Decodificar el body
				 * Guardar el reporte del body
				 * Confirmar con delivery.Ack(false) si se guardo
				 */
		}
	}
}


func connect(ctx context.Context, url, queue string) (*rabbitSession, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var socket net.Conn
	var stop func() bool
	connection, err := amqp.DialConfig(url, amqp.Config{
		Heartbeat: 5 * time.Second,
		Dial: func(network, address string) (net.Conn, error) {
			var err error
			socket, err = (&net.Dialer{}).DialContext(ctx, network, address)
			if err != nil {
				return nil, err
			}
			stop = context.AfterFunc(ctx, func() { _ = socket.Close() })
			return socket, nil
		},
	})
	if stop != nil {
		defer stop()
	}
	if err != nil {
		return nil, err
	}
	channel, err := connection.Channel()
	if err == nil {
		_, err = channel.QueueDeclare(queue, true, false, false, false, nil)
	}
	
	if err != nil {
		_ = socket.Close()
		return nil, err
	}
	return &rabbitSession{connection: connection, channel: channel, socket: socket}, nil
}

func main(){
	if err := run(); err != nil {
		log.Fatal(err)
	}
}