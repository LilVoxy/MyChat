// websocket/message_handler.go
package websocket

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"
)

// HandleMessage обрабатывает входящие сообщения
func (m *Manager) HandleMessage(message []byte, client *Client) {
	// Парсим сообщение
	var msg Message
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("❌ Ошибка разбора сообщения: %v", err)
		return
	}

	// Проверяем тип сообщения
	if msg.Type != "message" {
		log.Printf("❌ Неверный тип сообщения: %s", msg.Type)
		return
	}

	// Проверяем обязательные поля
	if msg.FromID == 0 || msg.ToID == 0 || msg.Content == "" {
		log.Printf("❌ Отсутствуют обязательные поля сообщения")
		return
	}

	// Устанавливаем текущее время, если не указано
	if msg.Timestamp == "" {
		msg.Timestamp = time.Now().Format("15:04")
	}

	// Сохраняем сообщение в базу данных
	chatID := msg.ChatID // Используем chat_id из сообщения, если он есть
	if chatID == 0 {
		// Если chat_id не указан, ищем или создаем чат
		var err error
		chatID, err = m.findOrCreateChat(msg.FromID, msg.ToID, msg.ProductID)
		if err != nil {
			log.Printf("❌ Ошибка при поиске/создании чата: %v", err)
			return
		}
	}

	// Добавляем chatId к сообщению для клиента
	msg.ChatID = chatID

	// Сохраняем сообщение в базу данных
	msgID, err := m.saveMessage(chatID, msg.FromID, msg.Content)
	if err != nil {
		log.Printf("❌ Ошибка при сохранении сообщения: %v", err)
		return
	}

	// Устанавливаем ID сообщения
	msg.ID = msgID

	// Отправляем сообщение получателю и отправителю
	m.sendMessageToClients(msg)

	// Обновляем последнее сообщение в чате
	go m.updateLastMessage(chatID, msg.Content)
}

// Находит существующий чат или создает новый
func (m *Manager) findOrCreateChat(fromID, toID, productID int) (int, error) {
	// Ищем существующий чат
	var chatID int
	err := m.DB.QueryRow(`
		SELECT id FROM chats
		WHERE ((buyer_id = ? AND seller_id = ?) OR (buyer_id = ? AND seller_id = ?))
		AND product_id = ?
		LIMIT 1
	`, fromID, toID, toID, fromID, productID).Scan(&chatID)

	if err == nil {
		// Чат найден, возвращаем его ID
		return chatID, nil
	}

	if err != sql.ErrNoRows {
		// Произошла ошибка при поиске
		return 0, err
	}

	// Определяем, кто покупатель, а кто продавец
	// В реальном приложении может быть сложная логика
	// Для упрощения считаем, что меньший ID - это продавец
	var buyerID, sellerID int
	if fromID < toID {
		sellerID = fromID
		buyerID = toID
	} else {
		sellerID = toID
		buyerID = fromID
	}

	// Создаем новый чат
	result, err := m.DB.Exec(`
		INSERT INTO chats (buyer_id, seller_id, product_id, created_at)
		VALUES (?, ?, ?, NOW())
	`, buyerID, sellerID, productID)

	if err != nil {
		return 0, err
	}

	// Получаем ID нового чата
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	log.Printf("✅ Создан новый чат ID: %d между покупателем %d и продавцом %d для товара %d",
		id, buyerID, sellerID, productID)

	return int(id), nil
}

// Сохраняет сообщение в базу данных
func (m *Manager) saveMessage(chatID, fromID int, content string) (int, error) {
	result, err := m.DB.Exec(`
		INSERT INTO messages (chat_id, sender_id, message, created_at, read_status)
		VALUES (?, ?, ?, NOW(), FALSE)
	`, chatID, fromID, content)

	if err != nil {
		return 0, err
	}

	// Получаем ID нового сообщения
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	log.Printf("✅ Сохранено сообщение ID: %d в чат ID: %d от пользователя %d: %s",
		id, chatID, fromID, content)

	return int(id), nil
}

// Отправляет сообщение клиентам через WebSocket
func (m *Manager) sendMessageToClients(msg Message) {
	// Получаем статус прочтения для исходящего сообщения
	var readStatus bool
	err := m.DB.QueryRow(`
		SELECT read_status FROM messages WHERE id = ?
	`, msg.ID).Scan(&readStatus)

	if err == nil {
		// Устанавливаем статус прочтения в сообщении
		msg.ReadStatus = readStatus
	} else {
		// Если ошибка, считаем что сообщение не прочитано
		msg.ReadStatus = false
		// Выводим ошибку только если это не "запись не найдена"
		if err != sql.ErrNoRows {
			log.Printf("⚠️ Ошибка при получении статуса прочтения: %v", err)
		}
	}

	// Сериализуем сообщение в JSON
	messageJSON, err := json.Marshal(msg)
	if err != nil {
		log.Printf("❌ Ошибка при сериализации сообщения: %v", err)
		return
	}

	// Отправляем получателю
	if client, ok := m.Clients[msg.ToID]; ok {
		client.Send <- messageJSON
		log.Printf("✅ Сообщение отправлено получателю: %d (статус прочтения: %v)", msg.ToID, msg.ReadStatus)
	} else {
		log.Printf("⚠️ Получатель %d не в сети", msg.ToID)
	}

	// Отправляем отправителю (для синхронизации между устройствами)
	if msg.FromID != msg.ToID { // Избегаем дублирования для сообщений самому себе
		if client, ok := m.Clients[msg.FromID]; ok {
			client.Send <- messageJSON
			log.Printf("✅ Сообщение отправлено отправителю: %d (для синхронизации, статус прочтения: %v)", msg.FromID, msg.ReadStatus)
		}
	}
}

// Обновляет последнее сообщение в чате
func (m *Manager) updateLastMessage(chatID int, message string) {
	_, err := m.DB.Exec(`
		UPDATE chats
		SET last_message = ?, last_message_time = NOW()
		WHERE id = ?
	`, message, chatID)

	if err != nil {
		log.Printf("❌ Ошибка при обновлении последнего сообщения в чате: %v", err)
		return
	}

	log.Printf("✅ Обновлено последнее сообщение в чате ID: %d", chatID)
}

// Обновляет и отправляет статус прочтения для сообщений
func (m *Manager) SendReadStatusUpdates(userID, chatWithID int) {
	// Получаем список сообщений, которые были отмечены как прочитанные
	rows, err := m.DB.Query(`
		SELECT m.id, m.chat_id, m.sender_id, m.read_status, m.created_at
		FROM messages m
		JOIN chats c ON m.chat_id = c.id
		WHERE m.sender_id = ? AND m.read_status = TRUE
		AND m.chat_id IN (
			SELECT id FROM chats
			WHERE (buyer_id = ? AND seller_id = ?) 
			   OR (buyer_id = ? AND seller_id = ?)
		)
	`, chatWithID, userID, chatWithID, chatWithID, userID)

	if err != nil {
		log.Printf("❌ Ошибка при получении обновлений статуса: %v", err)
		return
	}
	defer rows.Close()

	// Создаем список сообщений для обновления
	var messages []Message
	for rows.Next() {
		var msg Message
		var readStatus bool
		var createdAt time.Time

		if err := rows.Scan(&msg.ID, &msg.ChatID, &msg.FromID, &readStatus, &createdAt); err != nil {
			log.Printf("❌ Ошибка при сканировании сообщения: %v", err)
			continue
		}

		msg.ToID = userID
		msg.Type = "message"
		msg.ReadStatus = readStatus
		// Важно - не устанавливаем content, чтобы клиент знал, что это только обновление статуса
		msg.Content = ""                          // Явно устанавливаем пустую строку
		msg.Timestamp = createdAt.Format("15:04") // Добавляем timestamp для корректного отображения на клиенте

		messages = append(messages, msg)
	}

	// Отправляем обновления статуса для каждого сообщения
	for _, msg := range messages {
		// Отправляем только отправителю (так как получатель уже видел сообщение)
		if client, ok := m.Clients[msg.FromID]; ok {
			if msgJSON, err := json.Marshal(msg); err == nil {
				client.Send <- msgJSON
				log.Printf("✅ Отправлено обновление статуса для сообщения ID: %d, FromID: %d, ToID: %d, ReadStatus: %v",
					msg.ID, msg.FromID, msg.ToID, msg.ReadStatus)
			}
		} else {
			log.Printf("⚠️ Клиент %d не в сети для получения обновления статуса", msg.FromID)
		}
	}
}
