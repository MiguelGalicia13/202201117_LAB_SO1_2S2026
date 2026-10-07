package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/MiguelGalicia13/202201117_LAB_SO1_2S2026/Proyecto3/internals/report"
	"github.com/MiguelGalicia13/202201117_LAB_SO1_2S2026/Proyecto3/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type grpcServer struct {
	proto.UnimplementedWarReportServiceServer
	writeURL   string
	httpClient *http.Client
}

func fromProto(request *proto.WarReportRequest) (report.Report, error) {
	var country string
	switch request.GetCountry() {
	case proto.Countries_COUNTRY_USA:
		country = "USA"
	case proto.Countries_COUNTRY_RUS:
		country = "RUS"
	case proto.Countries_COUNTRY_CHN:
		country = "CHN"
	case proto.Countries_COUNTRY_ESP:
		country = "ESP"
	case proto.Countries_COUNTRY_GTM:
		country = "GTM"
	default:
		return report.Report{}, fmt.Errorf("país inválido")
	}

	planes := request.GetWarplanesInAir()
	ships := request.GetWarshipsInWater()
	value := report.Report{
		Country:         country,
		WarplanesInAir:  planes,
		WarshipsInWater: ships,
		Timestamp:       request.GetTimestamp(),
	}
	return value, value.Validate()
}

func (s *grpcServer) SendReport(ctx context.Context, request *proto.WarReportRequest) (*proto.WarReportResponse, error) {
	value, err := fromProto(request)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil, status.Error(codes.Internal, "Could not code report to JSON")
	}
	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		s.writeURL,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "Could not create HTTP request")
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := s.httpClient.Do(httpRequest)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, status.FromContextError(ctxErr).Err()
		}

		log.Printf("rabbit-writer unavailable: %v", err)
		return nil, status.Error(codes.Unavailable, "rabbit-writer no disponible")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, status.Error(codes.Unavailable, "rabbit-writer no pudo publicar el reporte")
	}

	return &proto.WarReportResponse{
		Status: "REPORT_PUBLISHED_TO_RABBITMQ",
	}, nil
}

func main() {
	writerURL := os.Getenv("RABBIT_WRITER_URL")
	if writerURL == "" {
		writerURL = "http://127.0.0.1:8081/publish"
	}
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("could not listen: %v", err)
	}
	server := grpc.NewServer()
	proto.RegisterWarReportServiceServer(server, &grpcServer{
		writeURL:   writerURL,
		httpClient: &http.Client{Timeout: 2 * time.Second},
	})

	log.Printf("grpc-server escuchando en %s", listener.Addr())
	shutdown, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	go func() {
		<-shutdown.Done()
		server.GracefulStop()
	}()

	log.Printf("grpc-server escuchando en %s", listener.Addr())
	if err := server.Serve(listener); err != nil && err != grpc.ErrServerStopped {
		log.Fatalf("gRPC server failed: %v", err)
	}
}
