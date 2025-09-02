package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"slices"
	"sync"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	inventoryV1 "github.com/germangorelkin/example-go-services/shared/pkg/proto/inventory/v1"
)

var (
	ErrPartNotFound = errors.New("part not found")
)

const grpcPort = 50051

// InventoryStorage представляет потокобезопасное хранилище данных о деталях
type InventoryStorage struct {
	mu   sync.RWMutex
	data map[string]*inventoryV1.Part
}

func NewInventoryStorage() *InventoryStorage {
	return &InventoryStorage{
		data: make(map[string]*inventoryV1.Part),
	}
}

func (s *InventoryStorage) Get(uid string) (*inventoryV1.Part, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.data[uid]
	if !ok {
		return p, ErrPartNotFound
	}
	return p, nil
}

func (s *InventoryStorage) Set(part *inventoryV1.Part) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[part.Uuid] = part
}

func (s *InventoryStorage) Filters(filters *inventoryV1.PartsFilter) []*inventoryV1.Part {
	s.mu.RLock()
	defer s.mu.RUnlock()

	parts := make([]*inventoryV1.Part, 0)

	for _, part := range s.data {
		// uuid
		if len(filters.Uuids) > 0 {
			found := slices.Contains(filters.Uuids, part.Uuid)
			if !found {
				break
			}
		}

		// name
		if len(filters.Names) > 0 {
			found := slices.Contains(filters.Names, part.Name)
			if !found {
				break
			}
		}

		// categories
		if len(filters.Categories) > 0 {
			found := slices.Contains(filters.Categories, part.Category.String())
			if !found {
				break
			}
		}

		// manufacturer_countries
		if len(filters.ManufacturerCountries) > 0 {
			found := slices.Contains(filters.ManufacturerCountries, part.Manufacturer.Country)
			if !found {
				break
			}
		}

		// tags
		if len(filters.Tags) > 0 {
			found := false
			for _, tag := range filters.Tags {
				if slices.Contains(part.Tags, tag) {
					found = true
					break
				}
			}
			if !found {
				break
			}
		}

		parts = append(parts, part)
	}

	return parts
}

// InventoryService реализует методы сервиса InventoryService
type InventoryService struct {
	storage *InventoryStorage

	inventoryV1.UnimplementedInventoryServiceServer
}

func NewInventoryService() *InventoryService {
	return &InventoryService{
		storage: NewInventoryStorage(),
	}
}

func (s *InventoryService) GetPart(_ context.Context, req *inventoryV1.GetPartRequest) (*inventoryV1.GetPartResponse, error) {
	partUUID := req.GetUuid()

	part, err := s.storage.Get(partUUID)
	if err != nil {
		if errors.Is(ErrPartNotFound, err) {
			return nil, status.Errorf(codes.NotFound, "part with UUID %s not found", partUUID)
		}
	}

	return &inventoryV1.GetPartResponse{Part: part}, nil
}

func (s *InventoryService) ListParts(_ context.Context, req *inventoryV1.ListPartsRequest) (*inventoryV1.ListPartsResponse, error) {
	parts := s.storage.Filters(req.GetFilter())

	return &inventoryV1.ListPartsResponse{
		Parts: parts,
	}, nil
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
	service := NewInventoryService()

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
