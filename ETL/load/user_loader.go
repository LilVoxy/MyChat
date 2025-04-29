package load

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/models"
	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// UserLoader отвечает за загрузку данных в измерение пользователей
type UserLoader struct {
	db     *sql.DB
	logger *utils.ETLLogger
}

// NewUserLoader создает новый экземпляр UserLoader
func NewUserLoader(db *sql.DB, logger *utils.ETLLogger) *UserLoader {
	return &UserLoader{
		db:     db,
		logger: logger,
	}
}

// Load загружает данные пользователей в OLAP
func (l *UserLoader) Load(users []models.UserDimension) error {
	if len(users) == 0 {
		l.logger.Debug("Нет данных пользователей для загрузки")
		return nil
	}

	startTime := time.Now()
	l.logger.Info("Начало загрузки данных пользователей (всего: %d)", len(users))

	// Подготавливаем запрос для обновления/вставки в user_dimension
	stmt, err := l.db.Prepare(`
		INSERT INTO chat_analytics.user_dimension 
		(id, registration_date, days_active, total_chats, total_messages, 
		avg_response_time_minutes, activity_level, last_updated)
		VALUES (?, ?, ?, ?, ?, ?, ?, NOW())
		ON DUPLICATE KEY UPDATE
		days_active = VALUES(days_active),
		total_chats = VALUES(total_chats),
		total_messages = VALUES(total_messages),
		avg_response_time_minutes = VALUES(avg_response_time_minutes),
		activity_level = VALUES(activity_level),
		last_updated = NOW()
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

	// Обрабатываем каждого пользователя
	for _, user := range users {
		// Добавляем/обновляем запись в user_dimension
		_, err := txStmt.Exec(
			user.ID,
			user.RegistrationDate.Format("2006-01-02"),
			user.DaysActive,
			user.TotalChats,
			user.TotalMessages,
			user.AvgResponseTimeMinutes,
			user.ActivityLevel,
		)
		if err != nil {
			l.logger.Error("Ошибка при обновлении user_dimension для пользователя %d: %v", user.ID, err)
			errors++
			continue
		}

		processed++

		// Логируем прогресс каждые 100 пользователей
		if processed%100 == 0 {
			l.logger.Debug("Загружено %d из %d пользователей...", processed, len(users))
		}
	}

	// Если были ошибки, откатываем транзакцию
	if errors > 0 {
		tx.Rollback()
		return fmt.Errorf("произошло %d ошибок при загрузке данных пользователей", errors)
	}

	// Фиксируем транзакцию
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("ошибка при фиксации транзакции: %w", err)
	}

	duration := time.Since(startTime)
	l.logger.Info("Загрузка данных пользователей завершена. Загружено записей: %d. Длительность: %v", processed, duration)

	return nil
}
