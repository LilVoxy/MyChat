// websocket/client_messaging.go
package websocket

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"strconv"

	"github.com/LilVoxy/coursework_chat/processor"
)

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
