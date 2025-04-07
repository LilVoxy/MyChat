// main.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/LilVoxy/coursework_chat/websocket"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
)

// Структура для сообщений
type Message struct {
	ID        int       `json:"id"`
	FromID    int       `json:"fromId"`
	ToID      int       `json:"toId"`
	ProductID int       `json:"productId"`
	Content   string    `json:"content"`
	Timestamp string    `json:"timestamp"`
	CreatedAt time.Time `json:"createdAt"`
}

// Структура для информации о чате
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

// Ответ API для сообщений
type MessagesResponse struct {
	Messages []Message `json:"messages"`
}

// Ответ API для списка чатов
type ChatsResponse struct {
	Chats []ChatInfo `json:"chats"`
}

func main() {
	fmt.Println("Запуск сервера...")

	// Инициализация базы данных
	db, err := websocket.InitDB()
	if err != nil {
		log.Fatalf("❌ Не удалось инициализировать базу данных: %v", err)
	}
	defer db.Close()

	// Создаем новый менеджер WebSocket с подключением к БД
	wsManager := websocket.NewManager(db)
	websocket.SetManager(wsManager)

	// Запускаем менеджер WebSocket
	go wsManager.Run()

	// Создаем маршрутизатор
	router := mux.NewRouter()

	// Настраиваем CORS
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// Регистрируем обработчики
	router.HandleFunc("/ws/{userId}", wsManager.HandleConnections)
	router.HandleFunc("/api/status", wsManager.HandleStatus).Methods("POST", "OPTIONS")

	// Добавляем обработчик для получения списка чатов
	router.HandleFunc("/api/chats", func(w http.ResponseWriter, r *http.Request) {
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
	}).Methods("GET", "OPTIONS")

	// Добавляем обработчик для получения истории сообщений
	router.HandleFunc("/api/messages", func(w http.ResponseWriter, r *http.Request) {
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
	}).Methods("GET", "OPTIONS")

	// Статические файлы
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("public")))

	// Настраиваем сервер
	server := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Запускаем сервер в отдельной горутине
	go func() {
		log.Printf("✅ Сервер запущен на http://localhost%s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Ошибка запуска сервера: %v", err)
		}
	}()

	// Канал для сигналов завершения
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Ожидаем сигнал завершения
	<-stop
	log.Println("⚠️ Получен сигнал завершения, закрываем соединения...")

	// Закрываем соединение с базой данных
	if err := db.Close(); err != nil {
		log.Printf("❌ Ошибка закрытия соединения с БД: %v", err)
	} else {
		log.Println("✅ Соединение с БД закрыто")
	}

	log.Println("👋 Сервер остановлен")
}
