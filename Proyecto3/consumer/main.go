package main

import (
	"context"
	"crypto/rand"
	"encoding/json"

	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/MiguelGalicia13/202201117_LAB_SO1_2S2026/Proyecto3/internals/report"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"gonum.org/v1/gonum/stat"
)

type rabbitSession struct {
	connection *amqp.Connection
	channel    *amqp.Channel
	socket     net.Conn
	returns    <-chan amqp.Return
}

func run() error {
	url := os.Getenv("RABBITMQ_URL")
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
	opts, err := redis.ParseURL(valkey_url)
	if err != nil {
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
	if err != nil {
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
	rdb.Set(valkeyCtx, "system::spainCount", 0, 0)
	rdb.Set(valkeyCtx, "ranking::spainAirplanes", 0, 0)
	rdb.Set(valkeyCtx, "ranking::usAirplanes", 0, 0)
	rdb.Set(valkeyCtx, "ranking::russiaAirplanes", 0, 0)
	rdb.Set(valkeyCtx, "ranking::chinaAirplanes", 0, 0)
	rdb.Set(valkeyCtx, "ranking::gtmAirplanes", 0, 0)

	rdb.Set(valkeyCtx, "ranking::spainBoats", 0, 0)
	rdb.Set(valkeyCtx, "ranking::usBoats", 0, 0)
	rdb.Set(valkeyCtx, "ranking::russiaBoats", 0, 0)
	rdb.Set(valkeyCtx, "ranking::chinaBoats", 0, 0)
	rdb.Set(valkeyCtx, "ranking::gtmBoats", 0, 0)

	rdb.Set(valkeyCtx, "stream::WarplanesInAir", 0, 0)
	rdb.Set(valkeyCtx, "stream::WarshipsInWater", 0, 0)
	for {
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
				 * Info que se puede obtener de esta key
				 * Maximo cantidad de aviones reportados
				 * Minima cantidad de aviones reportados
				 * Maxima cantidad de barcos reportados
				 * Minima cantidad de barcos reportados
				 */
				var reporteSend report.Report
				err := json.Unmarshal(deliveriy.Body, &reporteSend)
				if err != nil {
					log.Println("[ERROR] UNMARSHAL FAILED", err)
					continue
				}
				if err := reporteSend.Validate(); err != nil {
					log.Println("[ERROR] VALIDATE FAILED", err)
					continue
				}
				id := rand.Text()
				rdb.HSet(valkeyCtx, "system::reports", id, string(deliveriy.Body))
				rdb.IncrBy(valkeyCtx, "stream::WarplanesInAir", int64(reporteSend.WarplanesInAir))
				rdb.IncrBy(valkeyCtx, "stream::WarshipsInWater", int64(reporteSend.WarshipsInWater))
				
				// Incrementar contador de reportes de España si el reporte es de España}
				// Llevar contador de naves segun el pais
				switch reporteSend.Country{
					case "ESP":
						rdb.Incr(valkeyCtx, "system::spainCount")
						rdb.IncrBy(valkeyCtx, "ranking::spainAirplanes", int64(reporteSend.WarplanesInAir))
						rdb.IncrBy(valkeyCtx, "ranking::spainBoats", int64(reporteSend.WarshipsInWater))
					case "US":
						rdb.IncrBy(valkeyCtx, "ranking::usAirplanes", int64(reporteSend.WarplanesInAir))
						rdb.IncrBy(valkeyCtx, "ranking::usBoats", int64(reporteSend.WarshipsInWater))
					case "CHN":
						rdb.IncrBy(valkeyCtx, "ranking::chinaAirplanes", int64(reporteSend.WarplanesInAir))
						rdb.IncrBy(valkeyCtx, "ranking::chinaBoats", int64(reporteSend.WarshipsInWater))
					case "RUS":
						rdb.IncrBy(valkeyCtx, "ranking::russiaAirplanes", int64(reporteSend.WarplanesInAir))
						rdb.IncrBy(valkeyCtx, "ranking::russiaBoats", int64(reporteSend.WarshipsInWater))
					case "GTM":
						rdb.IncrBy(valkeyCtx, "ranking::gtmAirplanes", int64(reporteSend.WarplanesInAir))
						rdb.IncrBy(valkeyCtx, "ranking::gtmBoats", int64(reporteSend.WarshipsInWater))
				}
				// Obtener registros para resto de datos
				reports, err := rdb.HGetAll(valkeyCtx, "system::reports").Result()
				if err != nil {
					log.Println("[ERROR] HGETALL FAILED", err)
				}
				if len(reports) == 0 {
					continue
				}
				// Obtener registros de aviones y barcos reportados para calculo de la moda
				modeAviones := getMode(reports, "aviones")
				modeBarcos := getMode(reports, "barcos")
				rdb.Set(valkeyCtx, "system::airplanesMode", modeAviones, 0)
				rdb.Set(valkeyCtx, "system::shipsMode", modeBarcos, 0)

		}
	}
}

func getMode(reports map[string]string, key string) float64 {
	transport := make([]float64, 0, len(reports))
	for content := range reports {
		var read report.Report
		err := json.Unmarshal([]byte(content), &read)
		if err != nil {
			log.Println("[ERROR] UNMARSHAL FAILED", err)
			continue
		}
		if key == "aviones" {
			transport = append(transport, float64(read.WarplanesInAir))
		} else {
			transport = append(transport, float64(read.WarshipsInWater))
		}
	}
	mode, _ := stat.Mode(transport, nil)
	return mode
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

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
