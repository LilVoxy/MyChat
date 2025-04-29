package extractors

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/models"
	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// MessageExtractor извлекает данные о сообщениях из OLTP БД
type MessageExtractor struct {
	db     *sql.DB
	logger *utils.ETLLogger
}

// NewMessageExtractor создает новый экземпляр MessageExtractor
func NewMessageExtractor(db *sql.DB, logger *utils.ETLLogger) *MessageExtractor {
	return &MessageExtractor{
		db:     db,
		logger: logger,
	}
}

// ExtractMessages извлекает данные о сообщениях
// Если указаны lastRunTime и lastMessageID, будут извлечены только сообщения, созданные после этого времени и с ID больше lastMessageID
func (e *MessageExtractor) ExtractMessages(lastRunTime time.Time, lastMessageID int, batchSize int) ([]models.MessageOLTP, error) {
	e.logger.Debug("Начало извлечения данных о сообщениях (lastRunTime: %v, lastMessageID: %d)", lastRunTime, lastMessageID)

	query := `
		SELECT id, chat_id, sender_id, message, created_at, read_status 
		FROM messages 
		WHERE created_at > ? AND id > ?
		ORDER BY id
		LIMIT ?
	`

	// Если lastRunTime равно нулевому времени и lastMessageID равен 0, извлекаем все сообщения
	params := []interface{}{lastRunTime, lastMessageID, batchSize}
	if lastRunTime.IsZero() && lastMessageID == 0 {
		query = `
			SELECT id, chat_id, sender_id, message, created_at, read_status 
			FROM messages
			ORDER BY id
			LIMIT ?
		`
		params = []interface{}{batchSize}
	} else if lastRunTime.IsZero() {
		// Только по ID
		query = `
			SELECT id, chat_id, sender_id, message, created_at, read_status 
			FROM messages
			WHERE id > ?
			ORDER BY id
			LIMIT ?
		`
		params = []interface{}{lastMessageID, batchSize}
	} else if lastMessageID == 0 {
		// Только по времени
		query = `
			SELECT id, chat_id, sender_id, message, created_at, read_status 
			FROM messages
			WHERE created_at > ?
			ORDER BY id
			LIMIT ?
		`
		params = []interface{}{lastRunTime, batchSize}
	}

	// Выполняем запрос
	rows, err := e.db.Query(query, params...)
	if err != nil {
		e.logger.Error("Ошибка при извлечении данных о сообщениях: %v", err)
		return nil, fmt.Errorf("ошибка запроса сообщений: %w", err)
	}
	defer rows.Close()

	// Обрабатываем результаты
	var messages []models.MessageOLTP
	for rows.Next() {
		var message models.MessageOLTP
		if err := rows.Scan(&message.ID, &message.ChatID, &message.SenderID, &message.Message, &message.CreatedAt, &message.ReadStatus); err != nil {
			e.logger.Error("Ошибка при обработке данных сообщения: %v", err)
			return nil, fmt.Errorf("ошибка обработки данных сообщения: %w", err)
		}
		messages = append(messages, message)
	}

	// Проверяем ошибки после итерации по результатам
	if err = rows.Err(); err != nil {
		e.logger.Error("Ошибка после итерации по сообщениям: %v", err)
		return nil, fmt.Errorf("ошибка после итерации по сообщениям: %w", err)
	}

	e.logger.Debug("Извлечено %d сообщений", len(messages))
	return messages, nil
}

// GetMessagesByChatID извлекает все сообщения по ID чата
func (e *MessageExtractor) GetMessagesByChatID(chatID int) ([]models.MessageOLTP, error) {
	e.logger.Debug("Извлечение сообщений для чата ID: %d", chatID)

	query := `
		SELECT id, chat_id, sender_id, message, created_at, read_status 
		FROM messages 
		WHERE chat_id = ?
		ORDER BY created_at
	`

	// Выполняем запрос
	rows, err := e.db.Query(query, chatID)
	if err != nil {
		e.logger.Error("Ошибка при извлечении сообщений для чата %d: %v", chatID, err)
		return nil, fmt.Errorf("ошибка запроса сообщений по ID чата: %w", err)
	}
	defer rows.Close()

	// Обрабатываем результаты
	var messages []models.MessageOLTP
	for rows.Next() {
		var message models.MessageOLTP
		if err := rows.Scan(&message.ID, &message.ChatID, &message.SenderID, &message.Message, &message.CreatedAt, &message.ReadStatus); err != nil {
			e.logger.Error("Ошибка при обработке данных сообщения: %v", err)
			return nil, fmt.Errorf("ошибка обработки данных сообщения: %w", err)
		}
		messages = append(messages, message)
	}

	// Проверяем ошибки после итерации по результатам
	if err = rows.Err(); err != nil {
		e.logger.Error("Ошибка после итерации по сообщениям: %v", err)
		return nil, fmt.Errorf("ошибка после итерации по сообщениям: %w", err)
	}

	e.logger.Debug("Извлечено %d сообщений для чата %d", len(messages), chatID)
	return messages, nil
}

// GetLastMessageUpdateTime получает время последнего обновления сообщений
func (e *MessageExtractor) GetLastMessageUpdateTime() (time.Time, error) {
	var lastUpdateTime time.Time

	// Получаем время последнего обновления из таблицы сообщений
	err := e.db.QueryRow("SELECT MAX(created_at) FROM messages").Scan(&lastUpdateTime)
	if err != nil {
		if err == sql.ErrNoRows {
			// Если нет сообщений, возвращаем нулевое время
			return time.Time{}, nil
		}
		e.logger.Error("Ошибка при получении времени последнего обновления сообщений: %v", err)
		return time.Time{}, fmt.Errorf("ошибка получения времени последнего обновления: %w", err)
	}

	return lastUpdateTime, nil
}

// GetLastMessageID получает ID последнего сообщения
func (e *MessageExtractor) GetLastMessageID() (int, error) {
	var lastMessageID int

	// Получаем максимальный ID из таблицы сообщений
	err := e.db.QueryRow("SELECT MAX(id) FROM messages").Scan(&lastMessageID)
	if err != nil {
		if err == sql.ErrNoRows {
			// Если нет сообщений, возвращаем 0
			return 0, nil
		}
		e.logger.Error("Ошибка при получении ID последнего сообщения: %v", err)
		return 0, fmt.Errorf("ошибка получения ID последнего сообщения: %w", err)
	}

	return lastMessageID, nil
}
