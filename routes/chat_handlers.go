// routes/chat_handlers.go
package routes

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
)

// ChatInfo структура для информации о чате
type ChatInfo struct {
	ID              int       `json:"id"`
	BuyerID         int       `json:"buyerId"`
	SellerID        int       `json:"sellerId"`
	ProductID       int       `json:"productId"`
	LastMessage     string    `json:"lastMessage"`
	LastMessageTime string    `json:"lastMessageTime"`
	UnreadCount     int       `json:"unreadCount"`
	CreatedAt       time.Time `json:"createdAt"`
}

// ChatsResponse структура ответа API для списка чатов
type ChatsResponse struct {
	Chats []ChatInfo `json:"chats"`
}

// GetChatsHandler обрабатывает запросы на получение списка чатов
func GetChatsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Получаем параметры запроса
		query := r.URL.Query()
		userIdStr := query.Get("userId")

		// Поддержка альтернативного формата параметра (user_id)
		if userIdStr == "" {
			userIdStr = query.Get("user_id")
		}

		// Проверяем параметры
		if userIdStr == "" {
			http.Error(w, "Отсутствует обязательный параметр userId или user_id", http.StatusBadRequest)
			return
		}

		// Преобразуем ID в число
		userId, err := strconv.Atoi(userIdStr)
		if err != nil {
			http.Error(w, "Неверный формат ID пользователя", http.StatusBadRequest)
			return
		}

		// Получаем список чатов пользователя
		rows, err := db.Query(`
			SELECT 
				c.id, 
				c.buyer_id, 
				c.seller_id, 
				c.product_id, 
				c.created_at,
				IFNULL(
					(SELECT message FROM messages 
					 WHERE chat_id = c.id 
					 ORDER BY created_at DESC LIMIT 1), 
					''
				) as last_message,
				IFNULL(
					(SELECT created_at FROM messages 
					 WHERE chat_id = c.id 
					 ORDER BY created_at DESC LIMIT 1),
					c.created_at
				) as last_message_time
			FROM chats c
			WHERE c.buyer_id = ? OR c.seller_id = ?
			ORDER BY last_message_time DESC
		`, userId, userId)

		if err != nil {
			log.Printf("❌ Ошибка при запросе чатов: %v", err)
			http.Error(w, "Ошибка при получении списка чатов", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// Создаем слайс для хранения чатов
		var chats []ChatInfo

		// Обрабатываем результаты запроса
		for rows.Next() {
			var chat ChatInfo
			var lastMessageTime time.Time

			// Сканируем данные строки
			err := rows.Scan(
				&chat.ID,
				&chat.BuyerID,
				&chat.SellerID,
				&chat.ProductID,
				&chat.CreatedAt,
				&chat.LastMessage,
				&lastMessageTime,
			)
			if err != nil {
				log.Printf("❌ Ошибка при сканировании чата: %v", err)
				continue
			}

			// Форматируем время последнего сообщения
			chat.LastMessageTime = lastMessageTime.Format("15:04")

			// Если сообщение не сегодня, добавляем дату
			if lastMessageTime.Day() != time.Now().Day() ||
				lastMessageTime.Month() != time.Now().Month() ||
				lastMessageTime.Year() != time.Now().Year() {
				chat.LastMessageTime = lastMessageTime.Format("02.01.2006 15:04")
			}

			// Добавляем чат в слайс
			chats = append(chats, chat)
		}

		// Проверяем ошибки после итерации
		if err = rows.Err(); err != nil {
			log.Printf("❌ Ошибка при итерации по чатам: %v", err)
			http.Error(w, "Ошибка при обработке чатов", http.StatusInternalServerError)
			return
		}

		// Подготавливаем ответ
		response := ChatsResponse{
			Chats: chats,
		}

		// Устанавливаем заголовок для JSON
		w.Header().Set("Content-Type", "application/json")

		// Кодируем и отправляем ответ
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("❌ Ошибка при кодировании JSON: %v", err)
			http.Error(w, "Ошибка при формировании ответа", http.StatusInternalServerError)
			return
		}

		log.Printf("✅ Отправлен список из %d чатов для пользователя %d", len(chats), userId)
	}
}
