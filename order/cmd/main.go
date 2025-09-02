package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	orderV1 "github.com/germangorelkin/example-go-services/shared/pkg/openapi/order/v1"
	inventoryV1 "github.com/germangorelkin/example-go-services/shared/pkg/proto/inventory/v1"
	paymentV1 "github.com/germangorelkin/example-go-services/shared/pkg/proto/payment/v1"
)

var ErrOrderNotFound = errors.New("order not found")

const (
	httpPort = "9090"
	// Таймауты для HTTP-сервера
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 10 * time.Second
)

type Order struct {
	OrderUUID       string   `json:"order_uuid"`
	UserUUID        string   `json:"user_uuid"`
	PartUuids       []string `json:"part_uuids"`
	TotalPrice      float64  `json:"total_price"`
	TransactionUUID *string  `json:"transaction_uuid"`
	PaymentMethod   *string  `json:"payment_method"`
	Status          string   `json:"status"`
}

// OrderStorage представляет потокобезопасное хранилище данных о заказах
type OrderStorage struct {
	mu   sync.RWMutex
	data map[string]Order
}

// NewOrderStorage создает новое хранилище данных о заказах
func NewOrderStorage() *OrderStorage {
	return &OrderStorage{
		data: make(map[string]Order),
	}
}

// Get
func (s *OrderStorage) Get(uuid string) (Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	order, exists := s.data[uuid]
	if !exists {
		return order, ErrOrderNotFound
	}
	return order, nil
}

// Set
func (s *OrderStorage) Set(order Order) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[order.OrderUUID] = order
}

// OrderHandler реализует интерфейс orderV1.Handler для обработки запросов к API погоды
type OrderHandler struct {
	storage           *OrderStorage
	inventroryService inventoryV1.InventoryServiceClient
	paymentService    paymentV1.PaymentServiceClient
}

// NewOrderHandler создает новый обработчик запросов к API
func NewOrderHandler(storage *OrderStorage) *OrderHandler {
	return &OrderHandler{
		storage:           storage,
		inventroryService: newInventoryService(),
		paymentService:    newPaymentService(),
	}
}

func newInventoryService() inventoryV1.InventoryServiceClient {
	conn, err := grpc.NewClient(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Panicf("failed to connect: %v\n", err)
	}

	return inventoryV1.NewInventoryServiceClient(conn)
}

func newPaymentService() paymentV1.PaymentServiceClient {
	conn, err := grpc.NewClient(
		"localhost:50052",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Panicf("failed to connect: %v\n", err)
	}

	return paymentV1.NewPaymentServiceClient(conn)
}

// CreateOrder обрабатывает запрос на создание нового заказа
func (h *OrderHandler) CreateOrder(ctx context.Context, req *orderV1.CreateOrderRequest) (orderV1.CreateOrderRes, error) {
	// Валидация входных данных
	if req.UserUUID == "" || len(req.PartUuids) == 0 {
		return &orderV1.BadRequestError{
			Code:    http.StatusBadRequest,
			Message: "user_uuid и part_uuids не должны быть пустыми",
		}, nil
	}

	// Создание заказа в хранилище
	order := Order{
		OrderUUID: uuid.NewString(),
		UserUUID:  req.UserUUID,
		PartUuids: append([]string{}, req.PartUuids...),
		Status:    "PENDING_PAYMENT",
	}

	// вызов inventory
	parts, err := h.inventroryService.ListParts(context.Background(), &inventoryV1.ListPartsRequest{
		Filter: &inventoryV1.PartsFilter{
			Uuids: order.PartUuids,
		},
	})
	if err != nil {
		log.Printf("failed to ListParts:%v", err)
		return &orderV1.InternalServerError{}, nil
	}
	// check parts
	if len(parts.Parts) != len(order.PartUuids) {
		return &orderV1.BadRequestError{}, nil
	}

	// calc total price
	for _, part := range parts.GetParts() {
		order.TotalPrice += part.Price
	}

	// save
	h.storage.Set(order)

	// Возвращаем успешный ответ
	return &orderV1.CreateOrderResponse{
		OrderUUID:  order.OrderUUID,
		TotalPrice: order.TotalPrice,
	}, nil
}

// Проводит оплату ранее созданного заказа.
func (h *OrderHandler) PayOrder(ctx context.Context, req *orderV1.PayOrderRequest, params orderV1.PayOrderParams) (orderV1.PayOrderRes, error) {
	order, err := h.storage.Get(params.OrderUUID)
	if err != nil {
		return &orderV1.NotFoundError{}, nil
	}

	// вызов payment
	payment := &paymentV1.PayOrderRequest{
		OrderUuid:     order.OrderUUID,
		UserUuid:      order.UserUUID,
		PaymentMethod: 1,
	}
	tx, err := h.paymentService.PayOrder(context.Background(), payment)
	if err != nil {
		log.Printf("failed to PayOrder:%v", err)
		return &orderV1.InternalServerError{}, nil
	}

	// set payment info
	paymentMethod := "1"
	order.TransactionUUID = &tx.TransactionUuid
	order.PaymentMethod = &paymentMethod
	order.Status = "PAID"

	// update order
	h.storage.Set(order)

	// Возвращаем успешный ответ (пустой в данном случае)
	return &orderV1.PayOrderResponse{TransactionUUID: *order.TransactionUUID}, nil
}

func (h *OrderHandler) GetOrderByUuid(ctx context.Context, params orderV1.GetOrderByUuidParams) (orderV1.GetOrderByUuidRes, error) {
	order, err := h.storage.Get(params.OrderUUID)
	if err != nil {
		return &orderV1.NotFoundError{}, nil
	}

	return &orderV1.GetOrderResponse{
		OrderUUID:       uuid.MustParse(order.OrderUUID),
		UserUUID:        uuid.MustParse(order.UserUUID),
		PartUuids:       []uuid.UUID{},
		TotalPrice:      0,
		TransactionUUID: orderV1.NewOptNilUUID(uuid.MustParse(*order.TransactionUUID)),
		PaymentMethod:   orderV1.NewOptNilGetOrderResponsePaymentMethod(orderV1.GetOrderResponsePaymentMethodCARD),
		Status:          "",
	}, nil
}

func (h *OrderHandler) CancelOrder(ctx context.Context, params orderV1.CancelOrderParams) (orderV1.CancelOrderRes, error) {
	order, err := h.storage.Get(params.OrderUUID)
	if err != nil {
		return &orderV1.NotFoundError{}, nil
	}

	// заказ уже оплачен и не может быть отменён
	if order.Status == "PAID" {
		return &orderV1.ConflictError{}, nil
	}

	// change status
	order.Status = "CANCELLED"

	// save order
	h.storage.Set(order)

	return &orderV1.CancelOrderNoContent{}, nil
}

func main() {
	// Создаем хранилище для данных о погоде
	storage := NewOrderStorage()

	// Создаем обработчик API погоды
	orderHandler := NewOrderHandler(storage)

	// Создаем OpenAPI сервер
	orderServer, err := orderV1.NewServer(orderHandler)
	if err != nil {
		log.Fatalf("ошибка создания сервера OpenAPI: %v", err)
	}

	// Инициализируем роутер Chi
	r := chi.NewRouter()

	// Добавляем middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(10 * time.Second))

	// Монтируем обработчики OpenAPI
	r.Mount("/", orderServer)

	// Запускаем HTTP-сервер
	server := &http.Server{
		Addr:              net.JoinHostPort("localhost", httpPort),
		Handler:           r,
		ReadHeaderTimeout: readHeaderTimeout, // Защита от Slowloris атак - тип DDoS-атаки, при которой
		// атакующий умышленно медленно отправляет HTTP-заголовки, удерживая соединения открытыми и истощая
		// пул доступных соединений на сервере. ReadHeaderTimeout принудительно закрывает соединение,
		// если клиент не успел отправить все заголовки за отведенное время.
	}

	// Запускаем сервер в отдельной горутине
	go func() {
		log.Printf("🚀 HTTP-сервер запущен на порту %s\n", httpPort)
		err = server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("❌ Ошибка запуска сервера: %v\n", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Завершение работы сервера...")

	// Создаем контекст с таймаутом для остановки сервера
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	err = server.Shutdown(ctx)
	if err != nil {
		log.Printf("❌ Ошибка при остановке сервера: %v\n", err)
	}

	log.Println("✅ Сервер остановлен")
}
