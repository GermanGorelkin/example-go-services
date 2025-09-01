package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	paymentV1 "github.com/germangorelkin/example-go-services/shared/pkg/proto/payment/v1"
)

const grpcPort = 50052

// paymentService реализует методы сервиса PaymentServiceServer
type paymentService struct {
	paymentV1.UnsafePaymentServiceServer
}

func (s *paymentService) PayOrder(_ context.Context, req *paymentV1.PayOrderRequest) (*paymentV1.PayOrderResponse, error) {
	orderUUID := req.GetOrderUuid()
	userUUID := req.GetUserUuid()
	paymentMethod := req.GetPaymentMethod().String()

	log.Printf("Pay order orderUUID:%s, userUUID:%s, paymentMethod:%s", orderUUID, userUUID, paymentMethod)

	transactionUUID := uuid.NewString()

	log.Printf("Оплата прошла успешно, transaction_uuid: %s", transactionUUID)

	return &paymentV1.PayOrderResponse{TransactionUuid: transactionUUID}, nil
}

func main() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		log.Printf("failed to listen: %v\n", err)
		return
	}
	defer func() {
		if cerr := lis.Close(); cerr != nil {
			log.Printf("failed to close listener: %v\n", cerr)
		}
	}()

	// Создаем gRPC сервер
	s := grpc.NewServer()

	// Регистрируем наш сервис
	service := &paymentService{}

	paymentV1.RegisterPaymentServiceServer(s, service)

	// Включаем рефлексию для отладки
	reflection.Register(s)

	go func() {
		log.Printf("🚀 gRPC server listening on %d\n", grpcPort)
		err = s.Serve(lis)
		if err != nil {
			log.Printf("failed to serve: %v\n", err)
			return
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("🛑 Shutting down gRPC server...")
	s.GracefulStop()
	log.Println("✅ Server stopped")
}
