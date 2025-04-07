// database/message.go
package database

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"
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

// Секретный ключ для шифрования/расшифровки сообщений в БД
// В реальном приложении должен храниться в защищенном месте (например, в переменных окружения)
var dbEncryptionKey = []byte("this-is-32-byte-key-for-AES-GCM!") // Ровно 32 байта

// encryptForDB шифрует сообщение перед сохранением в базу данных
func encryptForDB(plaintext string) (string, error) {
	// Преобразуем текст в байты
	plaintextBytes := []byte(plaintext)

	// Создаем AES-шифр с нашим ключом
	block, err := aes.NewCipher(dbEncryptionKey)
	if err != nil {
		return "", err
	}

	// Создаем GCM (Galois/Counter Mode)
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Создаем nonce (number used once) - должен быть уникальным для каждого сообщения
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Шифруем данные
	ciphertext := gcm.Seal(nonce, nonce, plaintextBytes, nil)

	// Кодируем в base64 для удобного хранения в БД
	encoded := base64.StdEncoding.EncodeToString(ciphertext)

	return encoded, nil
}

// decryptFromDB расшифровывает сообщение, полученное из базы данных
func decryptFromDB(encryptedText string) (string, error) {
	// Декодируем base64
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}

	// Создаем AES-шифр с нашим ключом
	block, err := aes.NewCipher(dbEncryptionKey)
	if err != nil {
		return "", err
	}

	// Создаем GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Проверяем длину шифротекста
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	// Извлекаем nonce и шифротекст
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Расшифровываем
	plaintextBytes, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintextBytes), nil
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

// GetMessagesByChat возвращает все сообщения для указанного чата
// Сообщения автоматически расшифровываются при извлечении из БД
func GetMessagesByChat(chatID int, limit, offset int) ([]Message, error) {
	rows, err := DB.Query(`
		SELECT id, chat_id, sender_id, message, created_at, read_status 
		FROM messages 
		WHERE chat_id = ? 
		ORDER BY created_at DESC 
		LIMIT ? OFFSET ?
	`, chatID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var encryptedMessage string

		// Считываем данные из БД, включая зашифрованное сообщение
		if err := rows.Scan(&msg.ID, &msg.ChatID, &msg.SenderID, &encryptedMessage, &msg.CreatedAt, &msg.ReadStatus); err != nil {
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

		messages = append(messages, msg)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
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

// GetUnreadMessageCount возвращает количество непрочитанных сообщений для пользователя
func GetUnreadMessageCount(userID int) (int, error) {
	// Найдем все чаты, в которых участвует пользователь
	rows, err := DB.Query(`
		SELECT c.id 
		FROM chats c 
		WHERE c.buyer_id = ? OR c.seller_id = ?
	`, userID, userID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var chatIDs []int
	for rows.Next() {
		var chatID int
		if err := rows.Scan(&chatID); err != nil {
			return 0, err
		}
		chatIDs = append(chatIDs, chatID)
	}

	if err = rows.Err(); err != nil {
		return 0, err
	}

	// Если чатов нет, возвращаем 0
	if len(chatIDs) == 0 {
		return 0, nil
	}

	// Создаем запрос для подсчета непрочитанных сообщений
	query := "SELECT COUNT(*) FROM messages WHERE chat_id IN (?) AND sender_id != ? AND read_status = FALSE"

	// Преобразуем массив ID чатов в строку для SQL запроса
	// Это простейшая реализация для примера. В реальности лучше использовать
	// специальные библиотеки для работы с IN-запросами или кастомные функции
	var args []interface{}
	inClause := "("
	for i, id := range chatIDs {
		if i > 0 {
			inClause += ","
		}
		inClause += "?"
		args = append(args, id)
	}
	inClause += ")"

	query = "SELECT COUNT(*) FROM messages WHERE chat_id IN " + inClause + " AND sender_id != ? AND read_status = FALSE"
	args = append(args, userID)

	var count int
	err = DB.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
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
