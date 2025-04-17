// websocket/constants.go
package websocket

import (
	"time"
)

// Константы для WebSocket-соединения
const (
	// Время ожидания записи сообщения клиенту
	writeWait = 10 * time.Second

	// Время ожидания сообщения от клиента
	pongWait = 60 * time.Second

	// Период отправки пинг-сообщений
	pingPeriod = (pongWait * 9) / 10

	// Максимальный размер сообщения
	maxMessageSize = 512 * 1024 // 512KB

	// Добавляем таймаут для определения неактивности
	inactivityTimeout = 65 * time.Second
)
