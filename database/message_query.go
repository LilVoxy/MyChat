// database/message_query.go
package database

import (
	"log"
)

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
