// websocket/client.go
package websocket

import (
	"crypto/rsa"
	"encoding/json"
	"log"
	"strconv"

	"github.com/LilVoxy/coursework_chat/processor"
)

// Структура для передачи зашифрованных данных в формате JSON
type EncryptedPayload struct {
	EncryptedKey string `json:"encrypted_key"`
	Message      string `json:"message"`
	RecipientID  string `json:"recipient_id"`
	ProductID    string `json:"product_id"`
}

// Структура для обычных сообщений
type MessagePayload struct {
	RecipientID string `json:"recipient_id"`
	Message     string `json:"message"`
	ProductID   string `json:"product_id"`
}

// Хранилище ключей пользователей (в реальной системе это должно быть более безопасное хранилище)
var userKeys = make(map[string]*rsa.PrivateKey)

// ReadMessages читает сообщения от клиента.
func (c *Client) ReadMessages() {
	defer func() {
		log.Printf("Client %d disconnected", c.ID)
		close(c.Send)
		c.Socket.Close()
	}()

	// Генерируем ключевую пару для клиента при подключении
	privateKey, err := processor.GenerateKeyPair()
	if err != nil {
		log.Printf("Failed to generate keys for client %d: %v", c.ID, err)
		return
	}
	// Сохраняем приватный ключ в хранилище
	userKeys[strconv.Itoa(c.ID)] = privateKey

	for {
		_, message, err := c.Socket.ReadMessage()
		if err != nil {
			log.Printf("Client %d disconnected: %v", c.ID, err)
			break
		}

		// Проверяем, является ли сообщение зашифрованным JSON
		var encryptedPayload EncryptedPayload
		if err := json.Unmarshal(message, &encryptedPayload); err == nil && encryptedPayload.EncryptedKey != "" && encryptedPayload.Message != "" {
			// Сообщение зашифровано, обрабатываем его
			c.HandleEncryptedMessage(encryptedPayload)
		} else {
			// Пробуем распарсить как обычное JSON-сообщение
			var msgPayload MessagePayload
			if err := json.Unmarshal(message, &msgPayload); err == nil && msgPayload.RecipientID != "" && msgPayload.Message != "" {
				c.HandleJSONMessage(msgPayload)
			} else {
				// Обычное текстовое сообщение (незашифрованное)
				log.Printf("Received message from client %d: %s", c.ID, string(message))
				c.HandleTextMessage(message)
			}
		}
	}
}
