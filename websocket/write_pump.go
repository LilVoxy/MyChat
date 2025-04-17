// websocket/write_pump.go
package websocket

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// writePump отвечает за отправку сообщений клиенту
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		// Обработка паники при закрытии канала
		if r := recover(); r != nil {
			log.Printf("Паника при отправке сообщений клиенту %d: %v", c.ID, r)
		}

		ticker.Stop()

		// Безопасно закрываем соединение
		c.Socket.Close()

		log.Printf("Завершение writePump для клиента %d", c.ID)
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Socket.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Канал закрыт
				c.Socket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Отправляем каждое сообщение отдельно, без добавления newline
			// Это решает проблему с парсингом JSON на клиенте
			if err := c.Socket.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

			// Отправляем оставшиеся сообщения отдельными WriteMessage вызовами
			n := len(c.Send)
			for i := 0; i < n; i++ {
				message := <-c.Send
				if err := c.Socket.WriteMessage(websocket.TextMessage, message); err != nil {
					return
				}
			}
		case <-ticker.C:
			c.Socket.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Socket.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
