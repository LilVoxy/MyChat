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
	// Сериализуем сообщение в JSON
	messageJSON, err := json.Marshal(msg)
	if err != nil {
		log.Printf("❌ Ошибка при сериализации сообщения: %v", err)
		return
	}

	// Отправляем получателю
	if client, ok := m.Clients[msg.ToID]; ok {
		client.Send <- messageJSON
		log.Printf("✅ Сообщение отправлено получателю: %d", msg.ToID)
	} else {
		log.Printf("⚠️ Получатель %d не в сети", msg.ToID)
	}

	// Отправляем отправителю (для синхронизации между устройствами)
	if msg.FromID != msg.ToID { // Избегаем дублирования для сообщений самому себе
		if client, ok := m.Clients[msg.FromID]; ok {
			client.Send <- messageJSON
			log.Printf("✅ Сообщение отправлено отправителю: %d (для синхронизации)", msg.FromID)
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
