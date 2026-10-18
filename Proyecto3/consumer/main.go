package main

import (
	"context"
	"encoding/json"

	"fmt"
	"log"
	"net"
	"os"
	"time"
	"math"

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
	for {
		select {
		case <-rabbitCtx.Done():
			log.Println("[ERROR] RABBITMQ CONTEXT DONE")
			return nil
		case deliveriy, ok := <-deliveries:
			if !ok {
				log.Println("[ERROR] DELIVERIES CHANNEL CLOSED")
				return fmt.Errorf("DELIVERIES CHANNEL CLOSED")
			}
			log.Printf("[LOG] MESSAGE RECIEVED: %s, id: %s", deliveriy.Body, deliveriy.MessageId)
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
				return fmt.Errorf("could not unmarshal: %w", err)
			}
			if err := reporteSend.Validate(); err != nil {
				log.Println("[ERROR] VALIDATE FAILED", err)
				return fmt.Errorf("could not validate: %w", err)
			}
			id := deliveriy.MessageId
			if id == "" {
				log.Println("[ERROR] ID IS EMPTY")
				return fmt.Errorf("id is empty")
			}
			err = applyReport(valkeyCtx, rdb, id,deliveriy.Body, reporteSend)
			if err != nil{
				return fmt.Errorf("Couldnt Apply the rerport: %w", err)
			}
			// Obtener registros para resto de datos
			reports, err := rdb.HGetAll(valkeyCtx, "system::reports").Result()
			if err != nil {
				log.Println("[ERROR] HGETALL FAILED", err)
				return fmt.Errorf("Couldnt read system::reports: %w", err)
			}
			if len(reports) == 0 {
				return fmt.Errorf("el historial está vacío después de aplicar el reporte")
			}
			// Obtener registros de aviones y barcos reportados para calculo de la moda
			modeAviones := getMode(reports, "aviones")
			modeBarcos := getMode(reports, "barcos")
			err = rdb.Set(valkeyCtx, "system::airplanesMode", modeAviones, 0).Err()
			if err != nil {
				log.Println("[ERROR] COULD NOT SET system::airplanesMode", err)
				return fmt.Errorf("could not set system::airplanesMode: %w", err)
			}
			err = rdb.Set(valkeyCtx, "system::shipsMode", modeBarcos, 0).Err()
			if err != nil {
				log.Println("[ERROR] COULD NOT SET system::shipsMode", err)
				return fmt.Errorf("could not set system::shipsMode: %w", err)
			}
			err = deliveriy.Ack(false)
			if err != nil {
				log.Println("[ERROR] COULD NOT ACK", err)
				return fmt.Errorf("could not ack: %w", err)
			}

		}
	}
}

func getMode(reports map[string]string, key string) float64 {
	transport := make([]float64, 0, len(reports))
	for _, content := range reports {
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


func applyReport(
	ctx context.Context,
	rdb *redis.Client,
	id string,
	body []byte,
	reporteSend report.Report,
) error {
	paises := map[string]string{
		"ESP": "spain",
		"USA": "us",
		"CHN": "china",
		"RUS": "russia",
		"GTM": "gtm",
	}
	pais, ok := paises[reporteSend.Country]
	if !ok {
		return fmt.Errorf("unknown country: %s", reporteSend.Country)
	}
	contadores := map[string]int64{
		"stream::WarplanesInAir": int64(reporteSend.WarplanesInAir),
		"stream::WarshipsInWater": int64(reporteSend.WarshipsInWater),
		"ranking::"+pais+"Airplanes": int64(reporteSend.WarplanesInAir),
		"ranking::"+pais+"Boats": int64(reporteSend.WarshipsInWater),
	}
	if reporteSend.Country == "ESP" {
		contadores["system::spainCount"] = 1
	}
	keys := []string{"system::reports"}
	for key := range contadores{
		keys = append(keys, key)
	}
	for try := 0; try < 5; try++{
		err := rdb.Watch(ctx, func(tx *redis.Tx)error{
				exists, err := tx.HExists(ctx, "system::reports", id).Result()
				if err != nil{
					return err
				}
				if exists{
					return nil
				}
				next := make(map[string]int64, len(contadores))
				for key, contador := range contadores{
					curr, err := tx.Get(ctx, key).Int64()
					if err != nil && err != redis.Nil{
						return fmt.Errorf("Key %s: %w", key, err)
					}

					if contador <0 || curr <0 || curr > math.MaxInt64-contador{
						return fmt.Errorf("Key out of range: %s", key)
					}
					next[key] = curr + contador
				}
				_,err  = tx.TxPipelined(ctx, func(pipe redis.Pipeliner)error{
					pipe.HSet(ctx, "system::reports",id,string(body))
					for key, value := range next{
						pipe.Set(ctx, key, value, 0)
					}
					return nil
				})
				return err
			}, keys...)
		if err == redis.TxFailedErr{
			continue
		}
		return err
	}
	return fmt.Errorf("Couldnt apply report due to unkown circuntances")
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
