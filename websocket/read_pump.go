// websocket/read_pump.go
package websocket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// readPump читает сообщения от клиента и обрабатывает их
func (c *Client) readPump() {
	// Откладываем закрытие до конца функции
	defer func() {
		c.Manager.Unregister <- c
		c.Conn.Close()
		log.Printf("WebSocket закрыт для пользователя ID: %d", c.UserID)
	}()

	// Настраиваем соединение
	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Бесконечный цикл чтения сообщений от клиента
	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket закрыт неожиданно: %v", err)
			}
			break
		}

		// Обновляем время последней активности
		c.LastActivity = time.Now()

		// Парсим сообщение для определения типа
		var data struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(message, &data); err != nil {
			log.Printf("Ошибка при разборе сообщения: %v", err)
			continue
		}

		// Обрабатываем разные типы сообщений
		switch data.Type {
		case "ping":
			// Просто обновляем время активности
			log.Printf("Получен пинг от пользователя ID: %d", c.UserID)

		case "message":
			// Вызываем обработчик сообщений из Manager
			c.Manager.HandleMessage(message, c)

		case "status":
			// Обрабатываем запрос на изменение статуса
			var statusData struct {
				Type   string `json:"type"`
				Status string `json:"status"`
			}
			if err := json.Unmarshal(message, &statusData); err != nil {
				log.Printf("Ошибка при разборе сообщения статуса: %v", err)
				continue
			}

			// Обновляем статус пользователя
			c.Manager.updateUserStatus(c.UserID, statusData.Status, true)
		}
	}
}
