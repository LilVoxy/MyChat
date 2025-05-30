package load

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/models"
	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// ActivityLoader отвечает за загрузку фактов активности
type ActivityLoader struct {
	db     *sql.DB
	logger *utils.ETLLogger
}

// NewActivityLoader создает новый экземпляр ActivityLoader
func NewActivityLoader(db *sql.DB, logger *utils.ETLLogger) *ActivityLoader {
	return &ActivityLoader{
		db:     db,
		logger: logger,
	}
}

// LoadDailyFacts загружает ежедневные факты активности в OLAP
func (l *ActivityLoader) LoadDailyFacts(facts []models.DailyActivityFact) error {
	if len(facts) == 0 {
		l.logger.Debug("Нет данных ежедневной активности для загрузки")
		return nil
	}

	startTime := time.Now()
	l.logger.Info("Начало загрузки фактов ежедневной активности (всего: %d)", len(facts))

	// Подготавливаем запрос для вставки/обновления фактов ежедневной активности
	stmt, err := l.db.Prepare(`
		INSERT INTO chat_analytics.daily_activity_facts (
			date_id, total_messages, total_new_chats, active_users, new_users,
			avg_messages_per_chat, avg_response_time_minutes, peak_hour, peak_hour_messages
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		total_messages = VALUES(total_messages),
		total_new_chats = VALUES(total_new_chats),
		active_users = VALUES(active_users),
		new_users = VALUES(new_users),
		avg_messages_per_chat = VALUES(avg_messages_per_chat),
		avg_response_time_minutes = VALUES(avg_response_time_minutes),
		peak_hour = VALUES(peak_hour),
		peak_hour_messages = VALUES(peak_hour_messages)
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

	// Обрабатываем каждый факт ежедневной активности
	for _, fact := range facts {
		// Вставляем/обновляем запись в daily_activity_facts
		_, err := txStmt.Exec(
			fact.DateID,
			fact.TotalMessages,
			fact.TotalNewChats,
			fact.ActiveUsers,
			fact.NewUsers,
			fact.AvgMessagesPerChat,
			fact.AvgResponseTimeMinutes,
			fact.PeakHour,
			fact.PeakHourMessages,
		)

		if err != nil {
			l.logger.Error("Ошибка при обновлении daily_activity_facts для даты %d: %v", fact.DateID, err)
			errors++
			continue
		}

		processed++
	}

	// Если были ошибки, откатываем транзакцию
	if errors > 0 {
		tx.Rollback()
		return fmt.Errorf("произошло %d ошибок при загрузке фактов ежедневной активности", errors)
	}

	// Фиксируем транзакцию
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("ошибка при фиксации транзакции: %w", err)
	}

	duration := time.Since(startTime)
	l.logger.Info("Загрузка фактов ежедневной активности завершена. Загружено записей: %d. Длительность: %v", processed, duration)

	return nil
}
