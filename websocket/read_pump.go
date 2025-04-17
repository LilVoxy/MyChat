// websocket/read_pump.go
package websocket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// readPump обрабатывает чтение сообщений от клиента
func (c *Client) readPump(manager *Manager) {
	defer func() {
		// Обработка паники при закрытии канала
		if r := recover(); r != nil {
			log.Printf("Паника при чтении сообщений клиента %d: %v", c.ID, r)
		}

		// Обновляем статус при отключении
		manager.statusMutex.Lock()
		if status, exists := manager.UserStatuses[c.ID]; exists {
			status.Connected = false
			status.LastSeen = time.Now()
		}
		manager.statusMutex.Unlock()

		manager.updateUserStatus(c.ID, "offline", false)
		log.Printf("❌ Пользователь %d отключился", c.ID)

		// Отправляем сигнал отключения
		manager.Unregister <- c

		// Безопасно закрываем соединение
		c.Socket.Close()

		log.Printf("Завершение readPump для клиента %d", c.ID)
	}()

	// Устанавливаем параметры подключения
	c.Socket.SetReadLimit(maxMessageSize)
	c.Socket.SetReadDeadline(time.Now().Add(pongWait))
	c.Socket.SetPongHandler(func(string) error {
		c.Socket.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		// Читаем сообщения
		_, message, err := c.Socket.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Ошибка: %v", err)
			}
			break
		}

		// Обрабатываем полученное сообщение
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Println("Ошибка декодирования сообщения:", err)
			continue
		}

		// Устанавливаем ID отправителя из данных соединения
		msg.FromID = c.ID

		switch msg.Type {
		case "ping":
			// Отправляем понг-сообщение обратно клиенту
			pongMsg := Message{
				Type: "pong",
			}
			if pongData, err := json.Marshal(pongMsg); err == nil {
				c.Send <- pongData
			}
			continue

		case "message":
			// Делегируем обработку сообщения HandleMessage
			manager.HandleMessage(c, message)
			continue

		case "status":
			// Обрабатываем сообщение о статусе
			manager.updateUserStatus(c.ID, msg.Status, msg.IsActive)
			log.Printf("Обновлен статус %s для пользователя %d", msg.Status, c.ID)
		}
	}
}
