// websocket/client.go
package websocket

import (
	"bytes"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"log"
	"strconv"
	"strings"

	"github.com/LilVoxy/coursework_chat/database"
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

// Глобальная переменная для доступа к менеджеру соединений
// var manager *Manager - Удаляем эту переменную, используем globalManager из server.go

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

// HandleEncryptedMessage обрабатывает зашифрованное сообщение
func (c *Client) HandleEncryptedMessage(payload EncryptedPayload) {
	// Декодируем из base64
	encryptedKey, err := base64.StdEncoding.DecodeString(payload.EncryptedKey)
	if err != nil {
		log.Printf("Error decoding encrypted key: %v", err)
		return
	}

	encryptedMessage, err := base64.StdEncoding.DecodeString(payload.Message)
	if err != nil {
		log.Printf("Error decoding encrypted message: %v", err)
		return
	}

	// Получаем приватный ключ получателя
	recipientID := payload.RecipientID
	recipientKey, exists := userKeys[recipientID]
	if !exists {
		log.Printf("Recipient key not found for user %s", recipientID)
		return
	}

	// Расшифровываем и распаковываем сообщение
	plaintext, err := processor.ProcessInboundMessage(encryptedKey, encryptedMessage, recipientKey)
	if err != nil {
		log.Printf("Error processing inbound message: %v", err)
		return
	}

	// Теперь у нас есть расшифрованное сообщение
	log.Printf("Decrypted message for client %s: %s", recipientID, string(plaintext))

	// Преобразуем senderID и recipientID в int для сохранения в БД
	senderID := c.ID
	recipientIDInt, err := strconv.Atoi(recipientID)
	if err != nil {
		log.Printf("Invalid recipient ID: %s", recipientID)
		return
	}

	// Получаем ID товара
	productID := 0 // Значение по умолчанию, если ID товара не указан
	if payload.ProductID != "" {
		productID, err = strconv.Atoi(payload.ProductID)
		if err != nil {
			log.Printf("Invalid product ID: %s", payload.ProductID)
			return
		}
	}

	// Убеждаемся, что оба пользователя существуют в базе
	if err := database.EnsureUserExists(senderID); err != nil {
		log.Printf("Ошибка при проверке sender_id: %v", err)
		return
	}
	if err := database.EnsureUserExists(recipientIDInt); err != nil {
		log.Printf("Ошибка при проверке recipient_id: %v", err)
		return
	}

	// Сохраняем расшифрованное сообщение в БД
	_, err = database.SaveMessage(senderID, recipientIDInt, productID, string(plaintext))
	if err != nil {
		log.Printf("Ошибка сохранения сообщения: %v", err)
		return
	}

	// Отправляем сообщение получателю (если он онлайн)
	SendEncryptedMessageToClient(strconv.Itoa(c.ID), recipientID, payload.ProductID, plaintext)
}

// HandleJSONMessage обрабатывает сообщение в формате JSON
func (c *Client) HandleJSONMessage(payload MessagePayload) {
	recipientID := payload.RecipientID
	message := payload.Message
	productIDStr := payload.ProductID

	// Получаем ID отправителя
	senderID := c.ID

	// Преобразуем recipientID в int
	recipientIDInt, err := strconv.Atoi(recipientID)
	if err != nil {
		log.Printf("Invalid recipient ID: %s", recipientID)
		return
	}

	// Получаем ID товара
	productID := 0 // Значение по умолчанию, если ID товара не указан
	if productIDStr != "" {
		productID, err = strconv.Atoi(productIDStr)
		if err != nil {
			log.Printf("Invalid product ID: %s", productIDStr)
			return
		}
	}

	// Убеждаемся, что оба пользователя существуют в базе
	if err := database.EnsureUserExists(senderID); err != nil {
		log.Printf("Ошибка при проверке sender_id: %v", err)
		return
	}
	if err := database.EnsureUserExists(recipientIDInt); err != nil {
		log.Printf("Ошибка при проверке recipient_id: %v", err)
		return
	}

	// Сохраняем сообщение в MySQL
	_, err = database.SaveMessage(senderID, recipientIDInt, productID, message)
	if err != nil {
		log.Printf("Ошибка сохранения сообщения: %v", err)
		return
	}

	// Шифруем и отправляем сообщение получателю (если он онлайн)
	SendEncryptedMessageToClient(strconv.Itoa(c.ID), recipientID, productIDStr, []byte(message))
}

// HandleTextMessage обрабатывает входящее незашифрованное текстовое сообщение
func (c *Client) HandleTextMessage(message []byte) {
	// В текстовом сообщении предполагаем формат: recipientID productID text
	parts := []string{}
	for _, part := range bytes.Split(message, []byte(" ")) {
		if len(part) > 0 {
			parts = append(parts, string(part))
		}
	}

	if len(parts) < 2 {
		log.Printf("Invalid message format from client %d: %s", c.ID, string(message))
		return
	}

	recipientID := parts[0]

	// Предполагаем, что третья часть - это ID товара (если есть)
	productIDStr := "0" // Значение по умолчанию
	var text string

	if len(parts) >= 3 {
		productIDStr = parts[1]
		text = strings.Join(parts[2:], " ")
	} else {
		// Если частей только 2, то ID товара не указан
		text = parts[1]
	}

	// Получаем ID отправителя
	senderID := c.ID

	// Преобразуем recipientID в int
	recipientIDInt, err := strconv.Atoi(recipientID)
	if err != nil {
		log.Printf("Invalid recipient ID: %s", recipientID)
		return
	}

	// Получаем ID товара
	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		log.Printf("Invalid product ID: %s", productIDStr)
		return
	}

	// Убеждаемся, что оба пользователя существуют в базе
	if err := database.EnsureUserExists(senderID); err != nil {
		log.Printf("Ошибка при проверке sender_id: %v", err)
		return
	}
	if err := database.EnsureUserExists(recipientIDInt); err != nil {
		log.Printf("Ошибка при проверке recipient_id: %v", err)
		return
	}

	// Сохраняем сообщение в базу данных
	_, err = database.SaveMessage(senderID, recipientIDInt, productID, text)
	if err != nil {
		log.Printf("Ошибка сохранения сообщения: %v", err)
		return
	}

	// Шифруем и отправляем сообщение получателю (если он онлайн)
	SendEncryptedMessageToClient(strconv.Itoa(c.ID), recipientID, productIDStr, []byte(text))
}

// SendEncryptedMessageToClient шифрует и отправляет сообщение конкретному клиенту.
func SendEncryptedMessageToClient(senderID, recipientID, productID string, plaintext []byte) {
	// Проверки на валидность ID
	recipientIDInt, err := strconv.Atoi(recipientID)
	if err != nil {
		log.Printf("Ошибка конвертации recipientID в int: %v", err)
		return
	}

	// Проверяем, что получатель онлайн и имеет ключ
	recipientKey, keyExists := userKeys[recipientID]
	if !keyExists {
		log.Printf("Key not available for client %s, saving message to DB", recipientID)
		return
	}

	// Получаем публичный ключ получателя
	publicKey := &recipientKey.PublicKey

	// Сжимаем и шифруем сообщение
	encryptedKey, encryptedMessage, err := processor.ProcessOutboundMessage(plaintext, publicKey)
	if err != nil {
		log.Printf("Error processing outbound message: %v", err)
		return
	}

	// Кодируем в base64 для передачи по JSON
	payload := EncryptedPayload{
		EncryptedKey: base64.StdEncoding.EncodeToString(encryptedKey),
		Message:      base64.StdEncoding.EncodeToString(encryptedMessage),
		RecipientID:  recipientID,
		ProductID:    productID,
	}

	// Сериализуем в JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error encoding JSON: %v", err)
		return
	}

	// Проверяем инициализацию manager
	if globalManager == nil {
		log.Printf("Manager is not initialized")
		return
	}

	// Отправляем зашифрованное сообщение через систему менеджера
	if client, ok := globalManager.Clients[recipientIDInt]; ok {
		select {
		case client.Send <- jsonData:
			log.Printf("Encrypted message sent to client %s", recipientID)
		default:
			log.Printf("Failed to send encrypted message to client %s", recipientID)
		}
	} else {
		log.Printf("Recipient %s not found", recipientID)
	}
}

// SendMessageToClient отправляет сообщение клиенту (обертка над SendEncryptedMessageToClient)
func SendMessageToClient(senderID, targetID string, message []byte) {
	// Используем "0" как значение ID товара по умолчанию
	SendEncryptedMessageToClient(senderID, targetID, "0", message)
}