// websocket/types.go
package websocket

import (
	"database/sql"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Структура сообщения для обмена через WebSocket
type Message struct {
	Type      string `json:"type"`
	FromID    int    `json:"fromId,omitempty"`
	ToID      int    `json:"toId,omitempty"`
	ProductID int    `json:"productId,omitempty"`
	Content   string `json:"content,omitempty"`
	UserID    int    `json:"userId,omitempty"`
	Status    string `json:"status,omitempty"`
	IsActive  bool   `json:"isActive,omitempty"`
	ID        int    `json:"id,omitempty"`
}

// Клиент WebSocket
type Client struct {
	ID     int
	Socket *websocket.Conn
	Send   chan []byte
}

// Добавляем структуру для хранения статусов пользователей
type UserStatus struct {
	Status       string    `json:"status"`
	LastPing     time.Time `json:"last_ping"`
	IsActive     bool      `json:"is_active"`
	Connected    bool      `json:"connected"`
	LastSeen     time.Time `json:"last_seen"`
	ConnectionID string    `json:"connection_id"`
}

// Менеджер WebSocket-соединений
type Manager struct {
	Clients      map[int]*Client
	Broadcast    chan []byte
	Register     chan *Client
	Unregister   chan *Client
	DB           *sql.DB
	UserStatuses map[int]*UserStatus
	statusMutex  sync.RWMutex
}

// Конфигурация WebSocket-соединения
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Разрешаем подключения с любого источника (для разработки)
	},
}

// Глобальная переменная для менеджера
var globalManager *Manager
