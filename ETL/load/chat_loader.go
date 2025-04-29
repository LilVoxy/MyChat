package load

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/models"
	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// ChatLoader отвечает за загрузку фактов чатов
type ChatLoader struct {
	db     *sql.DB
	logger *utils.ETLLogger
}

// NewChatLoader создает новый экземпляр ChatLoader
func NewChatLoader(db *sql.DB, logger *utils.ETLLogger) *ChatLoader {
	return &ChatLoader{
		db:     db,
		logger: logger,
	}
}

// Load загружает факты чатов в OLAP
func (l *ChatLoader) Load(chats []models.ChatFact) error {
	if len(chats) == 0 {
		l.logger.Debug("Нет данных чатов для загрузки")
		return nil
	}

	startTime := time.Now()
	l.logger.Info("Начало загрузки фактов чатов (всего: %d)", len(chats))

	// Подготавливаем запрос для вставки/обновления фактов чатов
	stmt, err := l.db.Prepare(`
		INSERT INTO chat_analytics.chat_facts 
		(chat_id, start_time_id, end_time_id, buyer_id, seller_id, 
		total_messages, buyer_messages, seller_messages, 
		avg_message_length, avg_response_time_minutes, chat_duration_hours)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		start_time_id = VALUES(start_time_id),
		end_time_id = VALUES(end_time_id),
		buyer_id = VALUES(buyer_id),
		seller_id = VALUES(seller_id),
		total_messages = VALUES(total_messages),
		buyer_messages = VALUES(buyer_messages),
		seller_messages = VALUES(seller_messages),
		avg_message_length = VALUES(avg_message_length),
		avg_response_time_minutes = VALUES(avg_response_time_minutes),
		chat_duration_hours = VALUES(chat_duration_hours)
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

	// Обрабатываем каждый чат
	for _, chat := range chats {
		// Вставляем/обновляем запись в chat_facts
		_, err := txStmt.Exec(
			chat.ChatID,
			chat.StartTimeID,
			chat.EndTimeID,
			chat.BuyerID,
			chat.SellerID,
			chat.TotalMessages,
			chat.BuyerMessages,
			chat.SellerMessages,
			chat.AvgMessageLength,
			chat.AvgResponseTimeMinutes,
			chat.ChatDurationHours,
		)

		if err != nil {
			l.logger.Error("Ошибка при обновлении chat_facts для чата %d: %v", chat.ChatID, err)
			errors++
			continue
		}

		processed++

		// Логируем прогресс каждые 50 чатов
		if processed%50 == 0 {
			l.logger.Debug("Загружено %d из %d чатов...", processed, len(chats))
		}
	}

	// Если были ошибки, откатываем транзакцию
	if errors > 0 {
		tx.Rollback()
		return fmt.Errorf("произошло %d ошибок при загрузке фактов чатов", errors)
	}

	// Фиксируем транзакцию
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("ошибка при фиксации транзакции: %w", err)
	}

	duration := time.Since(startTime)
	l.logger.Info("Загрузка фактов чатов завершена. Загружено записей: %d. Длительность: %v", processed, duration)

	return nil
}
