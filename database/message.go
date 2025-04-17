// database/message.go
package database

import (
	"database/sql"
	"log"
	"time"
)

// Message представляет сообщение в чате
type Message struct {
	ID         int
	ChatID     int
	SenderID   int
	Message    string
	CreatedAt  time.Time
	ReadStatus bool
}

// SaveMessage сохраняет сообщение в базе данных.
// Перед сохранением сообщения, функция получает или создает чат между
// отправителем и получателем, относящийся к указанному товару.
// Сообщение шифруется перед сохранением в БД.
func SaveMessage(senderID, recipientID, productID int, message string) (int, error) {
	// Получаем или создаем чат между пользователями для указанного товара
	chatID, err := GetOrCreateChat(senderID, recipientID, productID)
	if err != nil {
		log.Printf("Ошибка получения/создания чата: %v", err)
		return 0, err
	}

	// Шифруем сообщение перед сохранением в БД
	encryptedMessage, err := encryptForDB(message)
	if err != nil {
		log.Printf("Ошибка шифрования сообщения: %v", err)
		return 0, err
	}

	// Вставляем зашифрованное сообщение в базу данных
	result, err := DB.Exec(
		"INSERT INTO messages (chat_id, sender_id, message) VALUES (?, ?, ?)",
		chatID, senderID, encryptedMessage,
	)
	if err != nil {
		log.Printf("Ошибка сохранения сообщения: %v", err)
		return 0, err
	}

	// Получаем ID вставленного сообщения
	messageID, err := result.LastInsertId()
	if err != nil {
		log.Printf("Ошибка получения ID сообщения: %v", err)
		return 0, err
	}

	return int(messageID), nil
}

// MarkMessagesAsRead отмечает все сообщения в чате как прочитанные для указанного пользователя
func MarkMessagesAsRead(chatID, userID int) error {
	_, err := DB.Exec(`
		UPDATE messages 
		SET read_status = TRUE 
		WHERE chat_id = ? AND sender_id != ? AND read_status = FALSE
	`, chatID, userID)
	return err
}

// GetChatLastMessage возвращает последнее сообщение в чате
func GetChatLastMessage(chatID int) (*Message, error) {
	var msg Message
	var encryptedMessage string

	err := DB.QueryRow(`
		SELECT id, chat_id, sender_id, message, created_at, read_status 
		FROM messages 
		WHERE chat_id = ? 
		ORDER BY created_at DESC 
		LIMIT 1
	`, chatID).Scan(&msg.ID, &msg.ChatID, &msg.SenderID, &encryptedMessage, &msg.CreatedAt, &msg.ReadStatus)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	// Расшифровываем сообщение
	decryptedMessage, err := decryptFromDB(encryptedMessage)
	if err != nil {
		log.Printf("Ошибка расшифровки сообщения %d: %v", msg.ID, err)
		msg.Message = "[Ошибка расшифровки]" // Помечаем сообщение с ошибкой
	} else {
		msg.Message = decryptedMessage
	}

	return &msg, nil
}
