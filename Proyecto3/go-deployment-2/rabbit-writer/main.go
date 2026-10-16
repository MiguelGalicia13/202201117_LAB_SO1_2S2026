package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"github.com/google/uuid"

	"github.com/MiguelGalicia13/202201117_LAB_SO1_2S2026/Proyecto3/internals/report"
	amqp "github.com/rabbitmq/amqp091-go"
)

type rabbitSession struct {
	connection *amqp.Connection
	channel    *amqp.Channel
	socket     net.Conn
	returns    <-chan amqp.Return
}

type writer struct {
	mu      sync.Mutex
	session *rabbitSession
	queue   string
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
	if err == nil {
		err = channel.Confirm(false)
	}
	if err != nil {
		_ = socket.Close()
		return nil, err
	}
	returns := channel.NotifyReturn(make(chan amqp.Return, 1))
	return &rabbitSession{connection: connection, channel: channel, socket: socket, returns: returns}, nil
}

func (w *writer) reconnect(ctx context.Context, url string) {
	delay := time.Second
	for ctx.Err() == nil {
		session, err := connect(ctx, url, w.queue)
		if err != nil {
			log.Printf("RabbitMQ connection failed: %v", err)
		} else {
			closed := session.channel.NotifyClose(make(chan *amqp.Error, 1))
			w.mu.Lock()
			w.session = session
			w.mu.Unlock()
			log.Printf("RabbitMQ ready, queue=%s", w.queue)
			delay = time.Second
			select {
			case <-ctx.Done():
			case <-closed:
			}
			w.mu.Lock()
			if w.session == session {
				w.session = nil
			}
			w.mu.Unlock()
			_ = session.socket.Close()
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = min(delay*2, 10*time.Second)
	}
}

func (w *writer) publish(ctx context.Context, body []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	session := w.session
	if session == nil || session.connection.IsClosed() || session.channel.IsClosed() {
		return fmt.Errorf("RabbitMQ unavailable")
	}
	deadline, _ := ctx.Deadline()
	if err := session.socket.SetWriteDeadline(deadline); err != nil {
		return err
	}
	defer session.socket.SetWriteDeadline(time.Time{})
	id, err := uuid.NewV7()
	if err != nil {
		log.Fatalf("Error al generar UUID v7: %v", err)
	}
	confirmation, err := session.channel.PublishWithDeferredConfirmWithContext(
		ctx, "", w.queue, true, false,
		amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: body, MessageId: id.String()},
	)
	if err == nil {
		var ack bool
		ack, err = confirmation.WaitContext(ctx)
		if err == nil && !ack {
			return fmt.Errorf("RabbitMQ rejected the report")
		}
		if err == nil {
			select {
			case <-session.returns:
				err = fmt.Errorf("RabbitMQ could not route the report")
			default:
			}
		}
	}
	if err != nil {
		// No retry: a report with a missing confirmation may already be in the queue.
		w.session = nil
		_ = session.socket.Close()
	}
	return err
}

func (w *writer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /ready", func(response http.ResponseWriter, _ *http.Request) {
		w.mu.Lock()
		ready := w.session != nil && !w.session.connection.IsClosed() && !w.session.channel.IsClosed()
		w.mu.Unlock()
		if !ready {
			http.Error(response, "RabbitMQ unavailable", http.StatusServiceUnavailable)
			return
		}
		response.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /publish", func(response http.ResponseWriter, request *http.Request) {
		value, err := report.Decode(response, request)
		if err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		body, err := json.Marshal(value)
		if err != nil {
			http.Error(response, "Could not encode report", http.StatusInternalServerError)
			return
		}
		ctx, cancel := context.WithTimeout(request.Context(), 1500*time.Millisecond)
		defer cancel()
		if err := w.publish(ctx, body); err != nil {
			log.Printf("Report not confirmed: %v", err)
			http.Error(response, "Report not confirmed by RabbitMQ", http.StatusServiceUnavailable)
			return
		}
		response.WriteHeader(http.StatusOK)
	})
	return mux
}

func run() error {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		return fmt.Errorf("RABBITMQ_URL must be set")
	}
	if _, err := amqp.ParseURI(url); err != nil {
		return fmt.Errorf("RABBITMQ_URL must be a valid AMQP URL")
	}
	queue := os.Getenv("RABBITMQ_QUEUE")
	if queue == "" {
		queue = "military_reports"
	}
	if len(queue) > 255 {
		return fmt.Errorf("RABBITMQ_QUEUE must be at most 255 bytes")
	}
	brokerCtx, cancelBroker := context.WithCancel(context.Background())
	done := make(chan struct{})
	w := &writer{queue: queue}
	go func() {
		defer close(done)
		w.reconnect(brokerCtx, url)
	}()
	defer func() { cancelBroker(); <-done }()

	server := &http.Server{
		Addr: "127.0.0.1:8081", Handler: w.routes(), ReadHeaderTimeout: 5 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	log.Printf("rabbit-writer listening on %s", server.Addr)
	select {
	case err := <-serverErrors:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return err
	}
	if err := <-serverErrors; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
