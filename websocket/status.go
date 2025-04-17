// websocket/status.go
package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// updateUserStatus обновляет статус пользователя
func (manager *Manager) updateUserStatus(userID int, status string, isActive bool) {
	manager.statusMutex.Lock()
	defer manager.statusMutex.Unlock()

	if _, exists := manager.UserStatuses[userID]; !exists {
		manager.UserStatuses[userID] = &UserStatus{
			LastSeen: time.Now(),
		}
	}

	statusObj := manager.UserStatuses[userID]
	oldStatus := statusObj.Status

	// Обновляем статус только если он действительно изменился
	if statusObj.Status != status || statusObj.IsActive != isActive {
		statusObj.Status = status
		statusObj.IsActive = isActive
		statusObj.LastPing = time.Now()
		statusObj.LastSeen = time.Now()

		// Логируем изменение статуса
		log.Printf("📊 Статус пользователя %d изменен: %s -> %s (активен: %v)",
			userID, oldStatus, status, isActive)

		// Создаем сообщение о статусе
		statusMsg := Message{
			Type:   "status",
			UserID: userID,
			Status: status,
		}

		// Отправляем статус всем клиентам, кроме самого пользователя
		if data, err := json.Marshal(statusMsg); err == nil {
			for clientID, client := range manager.Clients {
				// Исключаем отправку статуса самому пользователю, чтобы избежать дублирования
				if clientID != userID {
					select {
					case client.Send <- data:
					default:
						close(client.Send)
						delete(manager.Clients, client.ID)
					}
				}
			}
		}
	}
}

// checkUserActivity проверяет активность пользователей и обновляет их статусы
func (manager *Manager) checkUserActivity() {
	for {
		time.Sleep(inactivityTimeout / 2) // Проверяем каждые 30 секунд

		manager.statusMutex.Lock()
		now := time.Now()

		for userID, status := range manager.UserStatuses {
			// Проверяем время последней активности
			timeSinceLastSeen := now.Sub(status.LastSeen)
			timeSinceLastPing := now.Sub(status.LastPing)

			// Логируем состояние пользователя
			log.Printf("👤 Проверка пользователя %d: статус=%s, активен=%v, последняя активность=%v назад, последний пинг=%v назад",
				userID, status.Status, status.IsActive, timeSinceLastSeen, timeSinceLastPing)

			// Если пользователь подключен и неактивен более 60 секунд
			if status.Connected && status.Status == "online" && timeSinceLastPing > 60*time.Second {
				// Помечаем пользователя как неактивного
				manager.updateUserStatus(userID, "away", false)
				log.Printf("⚠️ Пользователь %d помечен как неактивный", userID)
			}

			// Если пользователь не пинговал сервер более 120 секунд
			if status.Connected && timeSinceLastPing > 120*time.Second {
				// Помечаем как отключенного
				manager.updateUserStatus(userID, "offline", false)
				log.Printf("❌ Пользователь %d помечен как отключенный", userID)
			}
		}

		manager.statusMutex.Unlock()
	}
}

// HandleStatus обрабатывает HTTP запросы для обновления статуса пользователя
func (manager *Manager) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if msg.Type != "status" {
		http.Error(w, "Invalid message type", http.StatusBadRequest)
		return
	}

	// Обновляем статус с учетом активности через единый метод
	manager.updateUserStatus(msg.UserID, msg.Status, msg.IsActive)

	// Отправляем успешный ответ с подтверждением
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Возвращаем информацию о выполненном действии
	response := map[string]interface{}{
		"success": true,
		"message": "Status updated successfully",
		"userId":  msg.UserID,
		"status":  msg.Status,
	}

	json.NewEncoder(w).Encode(response)
}
