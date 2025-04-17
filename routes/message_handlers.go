// routes/message_handlers.go
package routes

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
)

// Message структура для сообщений
type Message struct {
	ID        int       `json:"id"`
	FromID    int       `json:"fromId"`
	ToID      int       `json:"toId"`
	ProductID int       `json:"productId"`
	Content   string    `json:"content"`
	Timestamp string    `json:"timestamp"`
	CreatedAt time.Time `json:"createdAt"`
}

// MessagesResponse структура ответа API для сообщений
type MessagesResponse struct {
	Messages []Message `json:"messages"`
}

// GetMessagesHandler обрабатывает запросы на получение истории сообщений
func GetMessagesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Получаем параметры запроса
		query := r.URL.Query()
		userIdStr := query.Get("userId")
		chatWithStr := query.Get("chatWith")

		// Поддержка альтернативных форматов параметров (u_id и chat_with)
		if userIdStr == "" {
			userIdStr = query.Get("u_id")
		}

		if chatWithStr == "" {
			chatWithStr = query.Get("chat_with")
		}

		// Проверяем параметры
		if userIdStr == "" || chatWithStr == "" {
			http.Error(w, "Отсутствуют обязательные параметры (userId/u_id и chatWith/chat_with)", http.StatusBadRequest)
			return
		}

		// Преобразуем ID в числа
		userId, err := strconv.Atoi(userIdStr)
		if err != nil {
			http.Error(w, "Неверный формат ID пользователя", http.StatusBadRequest)
			return
		}

		chatWith, err := strconv.Atoi(chatWithStr)
		if err != nil {
			http.Error(w, "Неверный формат ID собеседника", http.StatusBadRequest)
			return
		}

		// Получаем сообщения из базы данных
		rows, err := db.Query(`
			SELECT m.id, m.sender_id as from_id, 
				   CASE 
					   WHEN m.sender_id = ? THEN ? 
					   ELSE m.sender_id 
				   END as to_id, 
				   c.product_id, m.message as content, m.created_at
			FROM messages m
			JOIN chats c ON m.chat_id = c.id
			WHERE m.chat_id IN (
				SELECT id FROM chats
				WHERE (buyer_id = ? AND seller_id = ?) 
				   OR (buyer_id = ? AND seller_id = ?)
			)
			ORDER BY m.created_at ASC
		`, userId, chatWith, userId, chatWith, chatWith, userId)

		if err != nil {
			log.Printf("❌ Ошибка при запросе сообщений: %v", err)
			http.Error(w, "Ошибка при получении сообщений", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// Создаем слайс для хранения сообщений
		var messages []Message

		// Обрабатываем результаты запроса
		for rows.Next() {
			var msg Message
			var createdAt time.Time

			// Сканируем данные строки
			err := rows.Scan(&msg.ID, &msg.FromID, &msg.ToID, &msg.ProductID, &msg.Content, &createdAt)
			if err != nil {
				log.Printf("❌ Ошибка при сканировании сообщения: %v", err)
				continue
			}

			// Форматируем время
			msg.CreatedAt = createdAt
			msg.Timestamp = createdAt.Format("15:04")

			// Добавляем сообщение в слайс
			messages = append(messages, msg)
		}

		// Помечаем сообщения как прочитанные
		if len(messages) > 0 {
			_, err := db.Exec(`
				UPDATE messages m
				JOIN chats c ON m.chat_id = c.id
				SET m.read_status = TRUE
				WHERE m.sender_id != ?
				AND m.read_status = FALSE
				AND m.chat_id IN (
					SELECT id FROM chats
					WHERE (buyer_id = ? AND seller_id = ?) 
					   OR (buyer_id = ? AND seller_id = ?)
				)
			`, userId, userId, chatWith, chatWith, userId)

			if err != nil {
				log.Printf("❌ Ошибка при обновлении статуса прочтения: %v", err)
			} else {
				log.Printf("✅ Обновлен статус прочтения сообщений для чата между пользователями %d и %d", userId, chatWith)
			}
		}

		// Проверяем ошибки после итерации
		if err = rows.Err(); err != nil {
			log.Printf("❌ Ошибка при итерации по сообщениям: %v", err)
			http.Error(w, "Ошибка при обработке сообщений", http.StatusInternalServerError)
			return
		}

		// Подготавливаем ответ
		response := MessagesResponse{
			Messages: messages,
		}

		// Устанавливаем заголовок для JSON
		w.Header().Set("Content-Type", "application/json")

		// Кодируем и отправляем ответ
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("❌ Ошибка при кодировании JSON: %v", err)
			http.Error(w, "Ошибка при формировании ответа", http.StatusInternalServerError)
			return
		}

		log.Printf("✅ Отправлено %d сообщений для чата между пользователями %d и %d", len(messages), userId, chatWith)
	}
}
