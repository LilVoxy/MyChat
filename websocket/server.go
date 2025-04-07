// websocket/server.go
package websocket

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// Константы для WebSocket-соединения
const (
	// Время ожидания записи сообщения клиенту
	writeWait = 10 * time.Second

	// Время ожидания сообщения от клиента
	pongWait = 60 * time.Second

	// Период отправки пинг-сообщений
	pingPeriod = (pongWait * 9) / 10

	// Максимальный размер сообщения
	maxMessageSize = 512 * 1024 // 512KB

	// Добавляем таймаут для определения неактивности
	inactivityTimeout = 65 * time.Second
)

// Настройки базы данных
type DBInfo struct {
	Username string
	Password string
	Host     string
	Port     string
	Database string
}

// Структура сообщения для обмена через WebSocket
type Message struct {
	Type      string `json:"type"`
	FromID    int    `json:"fromId,omitempty"`
	ToID      int    `json:"toId,omitempty"`
	ProductID int    `json:"productId,omitempty"`
	Content   string `json:"content,omitempty"`
	UserID    int    `json:"userId,omitempty"`
	Status    string `json:"status,omitempty"`
	IsActive  bool   `json:"isActive,omitempty"`
}

// Клиент WebSocket
type Client struct {
	ID     int
	Socket *websocket.Conn
	Send   chan []byte
}

// Добавляем структуру для хранения статусов пользователей
type UserStatus struct {
	Status       string    `json:"status"`
	LastPing     time.Time `json:"last_ping"`
	IsActive     bool      `json:"is_active"`
	Connected    bool      `json:"connected"`
	LastSeen     time.Time `json:"last_seen"`
	ConnectionID string    `json:"connection_id"`
}

// Менеджер WebSocket-соединений
type Manager struct {
	Clients      map[int]*Client
	Broadcast    chan []byte
	Register     chan *Client
	Unregister   chan *Client
	DB           *sql.DB
	UserStatuses map[int]*UserStatus
	statusMutex  sync.RWMutex
}

// Конфигурация WebSocket-соединения
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Разрешаем подключения с любого источника (для разработки)
	},
}

// Глобальная переменная для менеджера
var globalManager *Manager

// Установка глобального менеджера
func SetManager(manager *Manager) {
	if manager != nil {
		globalManager = manager
		log.Println("Глобальный менеджер установлен")
	} else {
		log.Println("Ошибка: попытка установить nil менеджер")
	}
}

// Инициализация базы данных
func InitDB() (*sql.DB, error) {
	// Настройки для подключения к базе данных
	dbInfo := &DBInfo{
		Username: "root",
		Password: "Vjnbkmlf40782",
		Host:     "localhost",
		Port:     "3306",
		Database: "chatdb",
	}

	// Формируем строку подключения
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbInfo.Username,
		dbInfo.Password,
		dbInfo.Host,
		dbInfo.Port,
		dbInfo.Database,
	)

	// Устанавливаем соединение с базой данных
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Printf("❌ Ошибка подключения к БД: %v", err)
		return nil, err
	}

	// Проверяем соединение
	if err := db.Ping(); err != nil {
		log.Printf("❌ Ошибка проверки соединения с БД: %v", err)
		return nil, err
	}

	// Устанавливаем параметры пула соединений
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Println("✅ Успешное подключение к базе данных")

	// Проверяем существование необходимых таблиц
	if err := createTablesIfNotExist(db); err != nil {
		log.Printf("❌ Ошибка создания таблиц: %v", err)
		return nil, err
	}

	return db, nil
}

// Создание необходимых таблиц, если они не существуют
func createTablesIfNotExist(db *sql.DB) error {
	// SQL для создания таблицы чатов
	createChatsTable := `
	CREATE TABLE IF NOT EXISTS chats (
		id INT AUTO_INCREMENT PRIMARY KEY,
		buyer_id INT NOT NULL,
		seller_id INT NOT NULL,
		product_id INT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_participants (buyer_id, seller_id, product_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`

	// SQL для создания таблицы сообщений
	createMessagesTable := `
	CREATE TABLE IF NOT EXISTS messages (
		id INT AUTO_INCREMENT PRIMARY KEY,
		chat_id INT NOT NULL,
		sender_id INT NOT NULL,
		message TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (chat_id) REFERENCES chats(id),
		INDEX idx_chat_id (chat_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`

	// Выполняем создание таблиц
	if _, err := db.Exec(createChatsTable); err != nil {
		return fmt.Errorf("ошибка создания таблицы chats: %v", err)
	}

	if _, err := db.Exec(createMessagesTable); err != nil {
		return fmt.Errorf("ошибка создания таблицы messages: %v", err)
	}

	log.Println("✅ Структура базы данных проверена и актуализирована")
	return nil
}

// Создание нового менеджера WebSocket-соединений
func NewManager(db *sql.DB) *Manager {
	return &Manager{
		Broadcast:    make(chan []byte),
		Register:     make(chan *Client),
		Unregister:   make(chan *Client),
		Clients:      make(map[int]*Client),
		DB:           db,
		UserStatuses: make(map[int]*UserStatus),
	}
}

// Run запускает работу менеджера
func (manager *Manager) Run() {
	// Запускаем мониторинг активности пользователей
	go manager.checkUserActivity()

	for {
		select {
		case client := <-manager.Register:
			manager.Clients[client.ID] = client
			log.Printf("👤 Клиент %d подключился", client.ID)

		case client := <-manager.Unregister:
			if _, ok := manager.Clients[client.ID]; ok {
				delete(manager.Clients, client.ID)
				close(client.Send)
				log.Printf("👤 Клиент %d отключился", client.ID)

				// Обновляем статус на "offline" при отключении
				manager.updateUserStatus(client.ID, "offline", false)
			}

		case message := <-manager.Broadcast:
			// Обрабатываем входящее сообщение
			var msg Message
			if err := json.Unmarshal(message, &msg); err != nil {
				log.Printf("❌ Ошибка разбора сообщения: %v", err)
				continue
			}

			// Если это пинг-сообщение, обновляем время последней активности
			if msg.Type == "ping" {
				manager.statusMutex.Lock()
				if status, exists := manager.UserStatuses[msg.UserID]; exists {
					status.LastPing = time.Now()
					status.IsActive = msg.IsActive
					if !msg.IsActive && status.Status == "online" {
						status.Status = "away"
						// Отправляем обновление статуса всем клиентам
						statusMsg := Message{
							Type:   "status",
							UserID: msg.UserID,
							Status: "away",
						}
						if data, err := json.Marshal(statusMsg); err == nil {
							manager.broadcast(data)
						}
					}
				}
				manager.statusMutex.Unlock()
				continue
			}

			// Рассылаем сообщение всем подключенным клиентам
			for _, client := range manager.Clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(manager.Clients, client.ID)
				}
			}
		}
	}
}

// Обработчик WebSocket-соединений
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
		ID:     userId,
		Socket: conn,
		Send:   make(chan []byte, 256),
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

	manager.updateUserStatus(userId, "online", true)
	log.Printf("✅ Пользователь %d подключился с адреса %s", userId, r.RemoteAddr)

	// Отправляем новому клиенту статусы всех пользователей
	manager.statusMutex.RLock()
	for userID, status := range manager.UserStatuses {
		statusMsg := Message{
			Type:   "status",
			UserID: userID,
			Status: status.Status,
		}
		if statusData, err := json.Marshal(statusMsg); err == nil {
			client.Send <- statusData
		}
	}
	manager.statusMutex.RUnlock()

	// Запускаем горутины для чтения и отправки сообщений
	go client.readPump(manager)
	go client.writePump()
}

// Обработчик для чтения сообщений от клиента
func (c *Client) readPump(manager *Manager) {
	defer func() {
		// Обработка паники при закрытии канала
		if r := recover(); r != nil {
			log.Printf("Паника при чтении сообщений клиента %d: %v", c.ID, r)
		}

		// Обновляем статус при отключении
		manager.statusMutex.Lock()
		if status, exists := manager.UserStatuses[c.ID]; exists {
			status.Connected = false
			status.LastSeen = time.Now()
		}
		manager.statusMutex.Unlock()

		manager.updateUserStatus(c.ID, "offline", false)
		log.Printf("❌ Пользователь %d отключился", c.ID)

		// Отправляем сигнал отключения
		manager.Unregister <- c

		// Безопасно закрываем соединение
		c.Socket.Close()

		log.Printf("Завершение readPump для клиента %d", c.ID)
	}()

	// Устанавливаем параметры подключения
	c.Socket.SetReadLimit(maxMessageSize)
	c.Socket.SetReadDeadline(time.Now().Add(pongWait))
	c.Socket.SetPongHandler(func(string) error {
		c.Socket.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		// Читаем сообщения
		_, message, err := c.Socket.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Ошибка: %v", err)
			}
			break
		}

		// Обрабатываем полученное сообщение
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Println("Ошибка декодирования сообщения:", err)
			continue
		}

		// Устанавливаем ID отправителя из данных соединения
		msg.FromID = c.ID

		switch msg.Type {
		case "ping":
			// Отправляем понг-сообщение обратно клиенту
			pongMsg := Message{
				Type: "pong",
			}
			if pongData, err := json.Marshal(pongMsg); err == nil {
				c.Send <- pongData
			}
			continue

		case "message":
			// Сериализуем сообщение один раз
			messageData, err := json.Marshal(msg)
			if err != nil {
				log.Println("Ошибка кодирования сообщения:", err)
				continue
			}

			// Сохраняем сообщение в базу данных
			manager.saveMessage(msg)
			log.Printf("💾 Сохраняем сообщение в БД от %d к %d: %s", msg.FromID, msg.ToID, msg.Content)

			// Отправляем сообщение только получателю
			if recipient, ok := manager.Clients[msg.ToID]; ok {
				recipient.Send <- messageData
			}

			// Отправляем копию отправителю
			c.Send <- messageData
			continue

		case "status":
			// Обрабатываем сообщение о статусе
			statusMsg := Message{
				Type:   "status",
				UserID: c.ID,
				Status: msg.Status,
			}
			statusData, err := json.Marshal(statusMsg)
			if err != nil {
				log.Printf("Ошибка сериализации статуса: %v", err)
				continue
			}
			// Отправляем статус всем клиентам
			manager.broadcast(statusData)
			log.Printf("Отправлен статус %s для пользователя %d", msg.Status, c.ID)
		}
	}
}

// Обработчик для отправки сообщений клиенту
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		// Обработка паники при закрытии канала
		if r := recover(); r != nil {
			log.Printf("Паника при отправке сообщений клиенту %d: %v", c.ID, r)
		}

		ticker.Stop()

		// Безопасно закрываем соединение
		c.Socket.Close()

		log.Printf("Завершение writePump для клиента %d", c.ID)
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Socket.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Канал закрыт
				c.Socket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Отправляем каждое сообщение отдельно, без добавления newline
			// Это решает проблему с парсингом JSON на клиенте
			if err := c.Socket.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

			// Отправляем оставшиеся сообщения отдельными WriteMessage вызовами
			n := len(c.Send)
			for i := 0; i < n; i++ {
				message := <-c.Send
				if err := c.Socket.WriteMessage(websocket.TextMessage, message); err != nil {
					return
				}
			}
		case <-ticker.C:
			c.Socket.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Socket.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Сохранение сообщения в базу данных
func (manager *Manager) saveMessage(msg Message) {
	// Получаем или создаем ID чата
	chatID, err := manager.getChatID(msg.FromID, msg.ToID, msg.ProductID)
	if err != nil {
		log.Printf("❌ Ошибка получения ID чата: %v", err)
		return
	}

	// Подготовка запроса для вставки сообщения
	stmt, err := manager.DB.Prepare(`
		INSERT INTO messages (chat_id, sender_id, message, created_at, read_status)
		VALUES (?, ?, ?, NOW(), FALSE)
	`)
	if err != nil {
		log.Printf("❌ Ошибка подготовки запроса для сохранения сообщения: %v", err)
		return
	}
	defer stmt.Close()

	// Выполнение запроса
	result, err := stmt.Exec(chatID, msg.FromID, msg.Content)
	if err != nil {
		log.Printf("❌ Ошибка выполнения запроса для сохранения сообщения: %v", err)
		return
	}

	// Получение ID вставленной записи
	lastID, err := result.LastInsertId()
	if err != nil {
		log.Printf("❌ Ошибка получения ID сохраненного сообщения: %v", err)
		return
	}

	log.Printf("✅ Сообщение успешно сохранено в БД (ID: %d, Chat ID: %d, Статус: непрочитано)", lastID, chatID)
}

// Получение ID чата (создается, если не существует)
func (manager *Manager) getChatID(fromID, toID, productID int) (int, error) {
	var chatID int

	// Проверяем подключение к БД
	if manager.DB == nil {
		return 0, fmt.Errorf("❌ Отсутствует подключение к базе данных")
	}

	// Проверяем корректность входных данных
	if fromID <= 0 || toID <= 0 || productID <= 0 {
		return 0, fmt.Errorf("❌ Некорректные ID пользователей или товара: fromID=%d, toID=%d, productID=%d", fromID, toID, productID)
	}

	// Проверка существования чата
	err := manager.DB.QueryRow(`
		SELECT id FROM chats 
		WHERE (buyer_id = ? AND seller_id = ? AND product_id = ?)
		OR (buyer_id = ? AND seller_id = ? AND product_id = ?)
	`, fromID, toID, productID, toID, fromID, productID).Scan(&chatID)

	if err == nil {
		log.Printf("✅ Найден существующий чат (ID: %d)", chatID)
		return chatID, nil
	}

	if err != sql.ErrNoRows {
		log.Printf("❌ Ошибка при поиске существующего чата: %v", err)
		return 0, err
	}

	// Определяем роли (кто покупатель, кто продавец)
	// По умолчанию считаем отправителя покупателем, а получателя продавцом
	buyerID := fromID
	sellerID := toID

	// Проверяем товар, чтобы выяснить, кто продавец
	var realSellerID int
	err = manager.DB.QueryRow("SELECT seller_id FROM products WHERE id = ?", productID).Scan(&realSellerID)
	if err == nil && realSellerID > 0 {
		// Если отправитель является продавцом этого товара, меняем роли
		if realSellerID == fromID {
			buyerID = toID
			sellerID = fromID
		} else if realSellerID == toID {
			buyerID = fromID
			sellerID = toID
		}
		log.Printf("✅ Определены роли: покупатель=%d, продавец=%d (товар принадлежит продавцу %d)", buyerID, sellerID, realSellerID)
	} else if err != sql.ErrNoRows {
		log.Printf("⚠️ Не удалось определить продавца товара %d: %v, используем стандартные роли", productID, err)
	}

	// Создаем новый чат, если не существует
	stmt, err := manager.DB.Prepare(`
		INSERT INTO chats (buyer_id, seller_id, product_id, created_at)
		VALUES (?, ?, ?, NOW())
	`)
	if err != nil {
		log.Printf("❌ Ошибка подготовки запроса для создания чата: %v", err)
		return 0, err
	}
	defer stmt.Close()

	result, err := stmt.Exec(buyerID, sellerID, productID)
	if err != nil {
		log.Printf("❌ Ошибка выполнения запроса для создания чата: %v", err)
		return 0, err
	}

	newChatID, err := result.LastInsertId()
	if err != nil {
		log.Printf("❌ Ошибка получения ID нового чата: %v", err)
		return 0, err
	}

	log.Printf("✅ Создан новый чат (ID: %d, Покупатель: %d, Продавец: %d, Товар: %d)", newChatID, buyerID, sellerID, productID)
	return int(newChatID), nil
}

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
		manager.saveMessage(msg)

		// Отправляем сообщение получателю
		if recipient, ok := manager.Clients[msg.ToID]; ok {
			select {
			case recipient.Send <- messageData:
			default:
				close(recipient.Send)
				delete(manager.Clients, recipient.ID)
			}
		}

		// Отправляем копию отправителю для подтверждения
		client.Send <- messageData

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

// broadcast отправляет сообщение всем подключенным клиентам
func (manager *Manager) broadcast(message []byte) {
	for _, client := range manager.Clients {
		select {
		case client.Send <- message:
		default:
			close(client.Send)
			delete(manager.Clients, client.ID)
		}
	}
}

// Обновляем метод для обновления статуса пользователя
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

		// Добавляем информацию об активности
		if data, err := json.Marshal(statusMsg); err == nil {
			manager.broadcast(data)
		}
	}
}

// Добавляем метод для проверки активности пользователей
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

// Обновляем HandleStatus для учета активности
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

	// Обновляем статус с учетом активности
	manager.updateUserStatus(msg.UserID, msg.Status, true)

	w.WriteHeader(http.StatusOK)
}
