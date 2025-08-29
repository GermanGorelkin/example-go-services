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

	orderV1 "github.com/germangorelkin/example-go-services/shared/pkg/openapi/order/v1"
)

const (
	httpPort = "9090"
	// Таймауты для HTTP-сервера
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 10 * time.Second
)

// OrderStorage представляет потокобезопасное хранилище данных о погоде
type OrderStorage struct {
	mu sync.RWMutex
}

// NewWeatherStorage создает новое хранилище данных о погоде
func NewOrderStorage() *OrderStorage {
	return &OrderStorage{}
}

// CreateOrder создает новый заказ на основе выбранных пользователем деталей
func (s *OrderStorage) CreateOrder() {

}

// WeatherHandler реализует интерфейс weatherV1.Handler для обработки запросов к API погоды
type OrderHandler struct {
	storage *OrderStorage
}

// NewWeatherHandler создает новый обработчик запросов к API погоды
func NewOrderHandler(storage *OrderStorage) *OrderHandler {
	return &OrderHandler{
		storage: storage,
	}
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
	h.storage.CreateOrder()

	// Возвращаем успешный ответ (пустой в данном случае)
	return &orderV1.CreateOrderResponse{}, nil
}

// Проводит оплату ранее созданного заказа.
func (h *OrderHandler) PayOrder(ctx context.Context, req *orderV1.PayOrderRequest, params orderV1.PayOrderParams) (orderV1.PayOrderRes, error) {
	// Логика проведения оплаты заказа в хранилище
	// ...

	// Возвращаем успешный ответ (пустой в данном случае)
	return &orderV1.PayOrderResponse{}, nil
}

func (h *OrderHandler) GetOrderByUuid(ctx context.Context, params orderV1.GetOrderByUuidParams) (orderV1.GetOrderByUuidRes, error) {
	// Логика получения информации о заказе из хранилища
	// ...

	// Возвращаем успешный ответ (пустой в данном случае)
	return &orderV1.GetOrderResponse{}, nil
}

func (h *OrderHandler) CancelOrder(ctx context.Context, params orderV1.CancelOrderParams) (orderV1.CancelOrderRes, error) {
	// Логика отмены заказа в хранилище
	// ...

	// Возвращаем успешный ответ (пустой в данном случае)
	return &orderV1.CancelOrderNoContent{}, nil
}

func main() {
	// Создаем хранилище для данных о погоде
	storage := NewOrderStorage()

	// Создаем обработчик API погоды
	orderHandler := NewOrderHandler(storage)

	// Создаем OpenAPI сервер
	weatherServer, err := orderV1.NewServer(orderHandler)
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
	r.Mount("/", weatherServer)

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
