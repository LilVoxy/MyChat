// websocket/connection_handler.go
package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// HandleConnections обрабатывает WebSocket-соединения
func (manager *Manager) HandleConnections(w http.ResponseWriter, r *http.Request) {
	// Получаем ID пользователя из URL
	params := mux.Vars(r)
	userIdStr := params["userId"]
	log.Printf("Получен запрос на установку WebSocket с параметром userId=%s, полный URL: %s", userIdStr, r.URL.String())

	// Проверяем, что ID является числом
	userId, err := strconv.Atoi(userIdStr)
	if err != nil {
		log.Printf("Невалидный ID пользователя: %s, ошибка: %v", userIdStr, err)
		http.Error(w, "Невалидный ID пользователя", http.StatusBadRequest)
		return
	}

	log.Printf("Установлено соединение с пользователем ID: %d", userId)

	// Устанавливаем WebSocket-соединение
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Ошибка при установке WebSocket-соединения:", err)
		return
	}

	// Создаем нового клиента
	client := &Client{
		ID:           userId,
		UserID:       userId,
		Socket:       conn,
		Conn:         conn,
		Send:         make(chan []byte, 256),
		Manager:      manager,
		LastActivity: time.Now(),
	}

	// Если клиент с таким ID уже существует, отключаем его
	if existingClient, ok := manager.Clients[userId]; ok {
		log.Printf("Пользователь ID: %d уже подключен. Заменяем соединение.", userId)

		// Безопасно закрываем канал существующего клиента
		// Удаляем клиента из менеджера перед закрытием канала
		delete(manager.Clients, userId)

		// Закрываем соединение и канал
		existingClient.Socket.Close()
		// Закрываем канал только если он еще не закрыт
		select {
		case _, ok := <-existingClient.Send:
			if ok {
				close(existingClient.Send)
			}
		default:
			close(existingClient.Send)
		}
	}

	// Регистрируем клиента в менеджере
	manager.Register <- client

	// Обновляем статус пользователя при подключении
	manager.statusMutex.Lock()
	if status, exists := manager.UserStatuses[userId]; exists {
		status.Connected = true
		status.ConnectionID = r.RemoteAddr
		status.LastSeen = time.Now()
	}
	manager.statusMutex.Unlock()

	// Вызываем обновление статуса через централизованную функцию
	manager.updateUserStatus(userId, "online", true)
	log.Printf("✅ Пользователь %d подключился с адреса %s", userId, r.RemoteAddr)

	// Отправляем новому клиенту статусы всех пользователей (кроме него самого)
	manager.statusMutex.RLock()
	for userID, status := range manager.UserStatuses {
		// Не отправляем пользователю его собственный статус
		if userID != userId {
			statusMsg := Message{
				Type:   "status",
				UserID: userID,
				Status: status.Status,
			}
			if statusData, err := json.Marshal(statusMsg); err == nil {
				client.Send <- statusData
			}
		}
	}
	manager.statusMutex.RUnlock()

	// Запускаем горутины для чтения и отправки сообщений
	go client.readPump()
	go client.writePump()
}
