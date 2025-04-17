// websocket/message_handler.go
package websocket

import (
	"encoding/json"
	"log"
)

// HandleMessage обрабатывает входящие сообщения
func (manager *Manager) HandleMessage(client *Client, messageData []byte) {
	var msg Message
	if err := json.Unmarshal(messageData, &msg); err != nil {
		log.Printf("Ошибка разбора JSON: %v", err)
		return
	}

	switch msg.Type {
	case "message":
		// Сохраняем сообщение в базу данных
		savedMsg, err := manager.saveMessage(msg)
		if err != nil {
			log.Printf("❌ Ошибка сохранения сообщения: %v", err)

			// Отправляем уведомление об ошибке отправителю
			errorMsg := Message{
				Type:    "error",
				Content: "Не удалось сохранить сообщение",
			}

			errorData, _ := json.Marshal(errorMsg)
			client.Send <- errorData
			return
		}

		// Добавляем ID сохраненного сообщения и другие возможные метаданные
		// к оригинальному сообщению перед отправкой
		msg.ID = savedMsg.ID // Предполагаем, что saveMessage возвращает сообщение с ID

		// Создаем обновленное сообщение с ID и временной меткой
		updatedMsg, err := json.Marshal(msg)
		if err != nil {
			log.Printf("❌ Ошибка сериализации сообщения: %v", err)
			return
		}

		// Отправляем сообщение только получателю, если он онлайн
		if recipient, ok := manager.Clients[msg.ToID]; ok {
			select {
			case recipient.Send <- updatedMsg:
				log.Printf("✅ Сообщение доставлено получателю %d", msg.ToID)
			default:
				log.Printf("❌ Не удалось доставить сообщение получателю %d", msg.ToID)
				close(recipient.Send)
				delete(manager.Clients, recipient.ID)
			}
		} else {
			log.Printf("ℹ️ Получатель %d не в сети, сообщение сохранено", msg.ToID)
		}

		// Отправляем подтверждение отправителю (но не копию всего сообщения)
		confirmMsg := Message{
			Type:    "confirmation",
			ID:      msg.ID, // ID сообщения для идентификации
			Status:  "sent", // Статус "отправлено"
			Content: "",     // Не дублируем контент
		}

		confirmData, _ := json.Marshal(confirmMsg)
		client.Send <- confirmData

	case "status":
		// Создаем сообщение о статусе
		statusMsg := Message{
			Type:   "status",
			UserID: msg.UserID,
			Status: msg.Status,
		}

		// Сериализуем сообщение
		statusData, err := json.Marshal(statusMsg)
		if err != nil {
			log.Printf("Ошибка сериализации статуса: %v", err)
			return
		}

		// Отправляем статус всем подключенным клиентам
		manager.broadcast(statusData)
	}
}

// saveMessage сохраняет сообщение в базе данных
func (manager *Manager) saveMessage(msg Message) (Message, error) {
	// Получаем или создаем ID чата
	chatID, err := manager.getChatID(msg.FromID, msg.ToID, msg.ProductID)
	if err != nil {
		log.Printf("❌ Ошибка получения ID чата: %v", err)
		return msg, err
	}

	// Подготовка запроса для вставки сообщения
	stmt, err := manager.DB.Prepare(`
		INSERT INTO messages (chat_id, sender_id, message, created_at, read_status)
		VALUES (?, ?, ?, NOW(), FALSE)
	`)
	if err != nil {
		log.Printf("❌ Ошибка подготовки запроса для сохранения сообщения: %v", err)
		return msg, err
	}
	defer stmt.Close()

	// Выполнение запроса
	result, err := stmt.Exec(chatID, msg.FromID, msg.Content)
	if err != nil {
		log.Printf("❌ Ошибка выполнения запроса для сохранения сообщения: %v", err)
		return msg, err
	}

	// Получение ID вставленной записи
	lastID, err := result.LastInsertId()
	if err != nil {
		log.Printf("❌ Ошибка получения ID сохраненного сообщения: %v", err)
		return msg, err
	}

	log.Printf("✅ Сообщение успешно сохранено в БД (ID: %d, Chat ID: %d, Статус: непрочитано)", lastID, chatID)

	msg.ID = int(lastID)
	return msg, nil
}
