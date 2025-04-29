package extractors

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/models"
	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// ChatExtractor извлекает данные о чатах из OLTP БД
type ChatExtractor struct {
	db     *sql.DB
	logger *utils.ETLLogger
}

// NewChatExtractor создает новый экземпляр ChatExtractor
func NewChatExtractor(db *sql.DB, logger *utils.ETLLogger) *ChatExtractor {
	return &ChatExtractor{
		db:     db,
		logger: logger,
	}
}

// ExtractChats извлекает данные о чатах
// Если указана lastRunTime, будут извлечены только чаты, созданные после этого времени
func (e *ChatExtractor) ExtractChats(lastRunTime time.Time, batchSize int) ([]models.ChatOLTP, error) {
	e.logger.Debug("Начало извлечения данных о чатах")

	query := `
		SELECT id, buyer_id, seller_id, created_at 
		FROM chats 
		WHERE created_at > ?
		ORDER BY id
		LIMIT ?
	`

	// Если lastRunTime равно нулевому времени, извлекаем все чаты
	params := []interface{}{lastRunTime, batchSize}
	if lastRunTime.IsZero() {
		query = `
			SELECT id, buyer_id, seller_id, created_at 
			FROM chats
			ORDER BY id
			LIMIT ?
		`
		params = []interface{}{batchSize}
	}

	// Выполняем запрос
	rows, err := e.db.Query(query, params...)
	if err != nil {
		e.logger.Error("Ошибка при извлечении данных о чатах: %v", err)
		return nil, fmt.Errorf("ошибка запроса чатов: %w", err)
	}
	defer rows.Close()

	// Обрабатываем результаты
	var chats []models.ChatOLTP
	for rows.Next() {
		var chat models.ChatOLTP
		if err := rows.Scan(&chat.ID, &chat.BuyerID, &chat.SellerID, &chat.CreatedAt); err != nil {
			e.logger.Error("Ошибка при обработке данных чата: %v", err)
			return nil, fmt.Errorf("ошибка обработки данных чата: %w", err)
		}
		chats = append(chats, chat)
	}

	// Проверяем ошибки после итерации по результатам
	if err = rows.Err(); err != nil {
		e.logger.Error("Ошибка после итерации по чатам: %v", err)
		return nil, fmt.Errorf("ошибка после итерации по чатам: %w", err)
	}

	e.logger.Debug("Извлечено %d чатов", len(chats))
	return chats, nil
}

// GetChatsByIDs извлекает данные о чатах по их ID
func (e *ChatExtractor) GetChatsByIDs(chatIDs []int) (map[int]models.ChatOLTP, error) {
	if len(chatIDs) == 0 {
		return make(map[int]models.ChatOLTP), nil
	}

	// Формируем список ID для SQL запроса
	// Для упрощения используем простой подход с подстановкой конкретных значений
	// В реальном проекте лучше использовать параметризированный запрос
	query := "SELECT id, buyer_id, seller_id, created_at FROM chats WHERE id IN ("
	params := make([]interface{}, len(chatIDs))

	for i, id := range chatIDs {
		if i > 0 {
			query += ","
		}
		query += "?"
		params[i] = id
	}
	query += ")"

	// Выполняем запрос
	rows, err := e.db.Query(query, params...)
	if err != nil {
		e.logger.Error("Ошибка при извлечении данных о чатах по ID: %v", err)
		return nil, fmt.Errorf("ошибка запроса чатов по ID: %w", err)
	}
	defer rows.Close()

	// Обрабатываем результаты
	chatMap := make(map[int]models.ChatOLTP)
	for rows.Next() {
		var chat models.ChatOLTP
		if err := rows.Scan(&chat.ID, &chat.BuyerID, &chat.SellerID, &chat.CreatedAt); err != nil {
			e.logger.Error("Ошибка при обработке данных чата: %v", err)
			return nil, fmt.Errorf("ошибка обработки данных чата: %w", err)
		}
		chatMap[chat.ID] = chat
	}

	// Проверяем ошибки после итерации по результатам
	if err = rows.Err(); err != nil {
		e.logger.Error("Ошибка после итерации по чатам: %v", err)
		return nil, fmt.Errorf("ошибка после итерации по чатам: %w", err)
	}

	return chatMap, nil
}

// GetLastChatUpdateTime получает время последнего обновления чатов
func (e *ChatExtractor) GetLastChatUpdateTime() (time.Time, error) {
	var lastUpdateTime time.Time

	// Получаем время последнего обновления из таблицы чатов
	err := e.db.QueryRow("SELECT MAX(created_at) FROM chats").Scan(&lastUpdateTime)
	if err != nil {
		if err == sql.ErrNoRows {
			// Если нет чатов, возвращаем нулевое время
			return time.Time{}, nil
		}
		e.logger.Error("Ошибка при получении времени последнего обновления чатов: %v", err)
		return time.Time{}, fmt.Errorf("ошибка получения времени последнего обновления: %w", err)
	}

	return lastUpdateTime, nil
}
