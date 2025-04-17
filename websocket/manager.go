// websocket/manager.go
package websocket

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// Установка глобального менеджера
func SetManager(manager *Manager) {
	if manager != nil {
		globalManager = manager
		log.Println("Глобальный менеджер установлен")
	} else {
		log.Println("Ошибка: попытка установить nil менеджер")
	}
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
			// Рассылаем сообщение всем подключенным клиентам
			manager.broadcast(message)
		}
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
