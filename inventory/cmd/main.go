package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	inventoryV1 "github.com/germangorelkin/example-go-services/shared/pkg/proto/inventory/v1"
)

const grpcPort = 50051

// inventoryService реализует методы сервиса InventoryService
type inventoryService struct {
	inventoryV1.UnimplementedInventoryServiceServer
}

func (s *inventoryService) GetPart(_ context.Context, req *inventoryV1.GetPartRequest) (*inventoryV1.GetPartResponse, error) {
	part := &inventoryV1.Part{}

	return &inventoryV1.GetPartResponse{
		Part: part,
	}, nil
}

func (s *inventoryService) ListParts(_ context.Context, req *inventoryV1.ListPartsRequest) (*inventoryV1.ListPartsResponse, error) {
	parts := []*inventoryV1.Part{}

	return &inventoryV1.ListPartsResponse{
		Parts: parts,
	}, nil
}

// Create создает новое наблюдение НЛО
// func (s *ufoService) Create(_ context.Context, req *ufoV1.CreateRequest) (*ufoV1.CreateResponse, error) {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()

// 	// Генерируем UUID для нового наблюдения
// 	newUUID := uuid.NewString()

// 	sighting := &ufoV1.Sighting{
// 		Uuid:      newUUID,
// 		Info:      req.GetInfo(),
// 		CreatedAt: timestamppb.New(time.Now()),
// 	}

// 	s.sightings[newUUID] = sighting

// 	log.Printf("Создано наблюдение с UUID %s", newUUID)

// 	return &ufoV1.CreateResponse{
// 		Uuid: newUUID,
// 	}, nil
// }

// // Get возвращает наблюдение НЛО по UUID
// func (s *ufoService) Get(_ context.Context, req *ufoV1.GetRequest) (*ufoV1.GetResponse, error) {
// 	s.mu.RLock()
// 	defer s.mu.RUnlock()

// 	sighting, ok := s.sightings[req.GetUuid()]
// 	if !ok {
// 		return nil, status.Errorf(codes.NotFound, "sighting with UUID %s not found", req.GetUuid())
// 	}

// 	return &ufoV1.GetResponse{
// 		Sighting: sighting,
// 	}, nil
// }

// // Update обновляет существующее наблюдение НЛО
// func (s *ufoService) Update(_ context.Context, req *ufoV1.UpdateRequest) (*emptypb.Empty, error) {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()

// 	sighting, ok := s.sightings[req.GetUuid()]
// 	if !ok {
// 		return nil, status.Errorf(codes.NotFound, "sighting with UUID %s not found", req.GetUuid())
// 	}

// 	if req.UpdateInfo == nil {
// 		return nil, status.Error(codes.InvalidArgument, "update_info cannot be nil")
// 	}

// 	// Обновляем поля, только если они были установлены в запросе
// 	if req.GetUpdateInfo().ObservedAt != nil {
// 		sighting.Info.ObservedAt = req.GetUpdateInfo().ObservedAt
// 	}

// 	if req.GetUpdateInfo().Location != nil {
// 		sighting.Info.Location = req.GetUpdateInfo().Location.Value
// 	}

// 	if req.GetUpdateInfo().Description != nil {
// 		sighting.Info.Description = req.GetUpdateInfo().Description.Value
// 	}

// 	if req.GetUpdateInfo().Color != nil {
// 		sighting.Info.Color = req.GetUpdateInfo().Color
// 	}

// 	if req.GetUpdateInfo().Sound != nil {
// 		sighting.Info.Sound = req.GetUpdateInfo().Sound
// 	}

// 	if req.GetUpdateInfo().DurationSeconds != nil {
// 		sighting.Info.DurationSeconds = req.GetUpdateInfo().DurationSeconds
// 	}

// 	sighting.UpdatedAt = timestamppb.New(time.Now())

// 	return &emptypb.Empty{}, nil
// }

// // Delete удаляет наблюдение НЛО (мягкое удаление - устанавливает deleted_at)
// func (s *ufoService) Delete(_ context.Context, req *ufoV1.DeleteRequest) (*emptypb.Empty, error) {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()

// 	sighting, ok := s.sightings[req.GetUuid()]
// 	if !ok {
// 		return nil, status.Errorf(codes.NotFound, "sighting with UUID %s not found", req.GetUuid())
// 	}

// 	// Мягкое удаление - устанавливаем deleted_at
// 	sighting.DeletedAt = timestamppb.New(time.Now())

// 	return &emptypb.Empty{}, nil
// }

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
	service := &inventoryService{}

	inventoryV1.RegisterInventoryServiceServer(s, service)

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
