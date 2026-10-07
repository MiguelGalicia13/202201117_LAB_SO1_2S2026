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
	"syscall"
	"time"

	"github.com/MiguelGalicia13/202201117_LAB_SO1_2S2026/Proyecto3/internals/report"
	"github.com/MiguelGalicia13/202201117_LAB_SO1_2S2026/Proyecto3/proto"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	grpcCallTimeout     = 3 * time.Second
	httpShutdownTimeout = 5 * time.Second
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	address := os.Getenv("GRPC_SERVER_ADDR")
	if address == "" {
		address = "dns:///go-grpc-service:50051"
	}
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(
			`{"loadBalancingConfig":[{"round_robin":{}}]}`,
		),
	)
	if err != nil {
		return fmt.Errorf("create gRPC client: %w", err)
	}
	defer conn.Close()
	client := proto.NewWarReportServiceClient(conn)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /send", func(w http.ResponseWriter, r *http.Request) {
		value, err := report.Decode(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		request, err := toProto(value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), grpcCallTimeout)
		defer cancel()

		response, err := client.SendReport(ctx, request)
		if err != nil {
			http.Error(w, "falló la llamada gRPC", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("no se pudo escribir la respuesta: %v", err)
		}
	})

	server := &http.Server{
		Addr:              "127.0.0.1:8081",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", server.Addr, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serveUntilShutdown(ctx, server, listener)
}

func serveUntilShutdown(ctx context.Context, server *http.Server, listener net.Listener) error {
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- server.Serve(listener)
	}()

	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return fmt.Errorf("shut down HTTP server: %w", err)
	}

	if err := <-serveErrors; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP during shutdown: %w", err)
	}
	return nil
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func toProto(value report.Report) (*proto.WarReportRequest, error) {
	var country proto.Countries
	switch value.Country {
	case "USA":
		country = proto.Countries_COUNTRY_USA
	case "RUS":
		country = proto.Countries_COUNTRY_RUS
	case "CHN":
		country = proto.Countries_COUNTRY_CHN
	case "ESP":
		country = proto.Countries_COUNTRY_ESP
	case "GTM":
		country = proto.Countries_COUNTRY_GTM
	default:
		return nil, fmt.Errorf("país inválido")
	}

	return &proto.WarReportRequest{
		Country:         country,
		WarplanesInAir:  value.WarplanesInAir,
		WarshipsInWater: value.WarshipsInWater,
		Timestamp:       value.Timestamp,
	}, nil
}
