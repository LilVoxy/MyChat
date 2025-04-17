// websocket/client_handlers.go
package websocket

import (
	"bytes"
	"encoding/base64"
	"log"
	"strconv"
	"strings"

	"github.com/LilVoxy/coursework_chat/database"
	"github.com/LilVoxy/coursework_chat/processor"
)

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
