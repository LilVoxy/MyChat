package load

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/models"
	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// MessageLoader отвечает за загрузку фактов сообщений
type MessageLoader struct {
	db     *sql.DB
	logger *utils.ETLLogger
}

// NewMessageLoader создает новый экземпляр MessageLoader
func NewMessageLoader(db *sql.DB, logger *utils.ETLLogger) *MessageLoader {
	return &MessageLoader{
		db:     db,
		logger: logger,
	}
}

// Load загружает факты сообщений в OLAP
func (l *MessageLoader) Load(messages []models.MessageFact) error {
	if len(messages) == 0 {
		l.logger.Debug("Нет данных сообщений для загрузки")
		return nil
	}

	startTime := time.Now()
	l.logger.Info("Начало загрузки фактов сообщений (всего: %d)", len(messages))

	// Подготавливаем запрос для вставки/обновления фактов сообщений
	stmt, err := l.db.Prepare(`
		INSERT INTO chat_analytics.message_facts 
		(message_id, time_id, sender_id, recipient_id, chat_id, message_length, 
		response_time_minutes, is_first_in_chat)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		time_id = VALUES(time_id),
		sender_id = VALUES(sender_id),
		recipient_id = VALUES(recipient_id),
		chat_id = VALUES(chat_id),
		message_length = VALUES(message_length),
		response_time_minutes = VALUES(response_time_minutes),
		is_first_in_chat = VALUES(is_first_in_chat)
	`)
	if err != nil {
		return fmt.Errorf("ошибка при подготовке запроса: %w", err)
	}
	defer stmt.Close()

	// Начинаем транзакцию
	tx, err := l.db.Begin()
	if err != nil {
		return fmt.Errorf("ошибка при начале транзакции: %w", err)
	}

	// Подготавливаем запрос в транзакции
	txStmt := tx.Stmt(stmt)
	defer txStmt.Close()

	processed := 0
	errors := 0

	batchSize := 100
	batch := 0

	// Обрабатываем каждое сообщение
	for _, msg := range messages {
		// Вставляем/обновляем запись в message_facts
		_, err := txStmt.Exec(
			msg.MessageID,
			msg.TimeID,
			msg.SenderID,
			msg.RecipientID,
			msg.ChatID,
			msg.MessageLength,
			msg.ResponseTimeMinutes,
			msg.IsFirstInChat,
		)

		if err != nil {
			l.logger.Error("Ошибка при обновлении message_facts для сообщения %d: %v", msg.MessageID, err)
			errors++
			continue
		}

		processed++
		batch++

		// Если достигли размера пакета, фиксируем транзакцию и начинаем новую
		if batch >= batchSize {
			// Фиксируем текущую транзакцию
			err = tx.Commit()
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("ошибка при фиксации транзакции: %w", err)
			}

			// Логируем прогресс
			l.logger.Debug("Загружено %d из %d сообщений...", processed, len(messages))

			// Начинаем новую транзакцию
			tx, err = l.db.Begin()
			if err != nil {
				return fmt.Errorf("ошибка при начале новой транзакции: %w", err)
			}

			// Подготавливаем запрос в новой транзакции
			txStmt = tx.Stmt(stmt)

			// Сбрасываем счетчик пакета
			batch = 0
		}
	}

	// Если были ошибки, откатываем транзакцию
	if errors > 0 {
		tx.Rollback()
		return fmt.Errorf("произошло %d ошибок при загрузке фактов сообщений", errors)
	}

	// Фиксируем последнюю транзакцию, если остались необработанные данные
	if batch > 0 {
		err = tx.Commit()
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("ошибка при фиксации последней транзакции: %w", err)
		}
	}

	duration := time.Since(startTime)
	l.logger.Info("Загрузка фактов сообщений завершена. Загружено записей: %d. Длительность: %v", processed, duration)

	return nil
}
