package extractors

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/models"
	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// UserExtractor извлекает данные о пользователях из OLTP БД
type UserExtractor struct {
	db     *sql.DB
	logger *utils.ETLLogger
}

// NewUserExtractor создает новый экземпляр UserExtractor
func NewUserExtractor(db *sql.DB, logger *utils.ETLLogger) *UserExtractor {
	return &UserExtractor{
		db:     db,
		logger: logger,
	}
}

// ExtractUsers извлекает данные о пользователях
// Если указана lastRunTime, будут извлечены только пользователи, созданные после этого времени
func (e *UserExtractor) ExtractUsers(lastRunTime time.Time, batchSize int) ([]models.UserOLTP, error) {
	e.logger.Debug("Начало извлечения данных о пользователях")

	query := `
		SELECT id, created_at 
		FROM users 
		WHERE created_at > ?
		ORDER BY id
		LIMIT ?
	`

	// Если lastRunTime равно нулевому времени, извлекаем всех пользователей
	params := []interface{}{lastRunTime, batchSize}
	if lastRunTime.IsZero() {
		query = `
			SELECT id, created_at 
			FROM users 
			ORDER BY id
			LIMIT ?
		`
		params = []interface{}{batchSize}
	}

	// Выполняем запрос
	rows, err := e.db.Query(query, params...)
	if err != nil {
		e.logger.Error("Ошибка при извлечении данных о пользователях: %v", err)
		return nil, fmt.Errorf("ошибка запроса пользователей: %w", err)
	}
	defer rows.Close()

	// Обрабатываем результаты
	var users []models.UserOLTP
	for rows.Next() {
		var user models.UserOLTP
		if err := rows.Scan(&user.ID, &user.CreatedAt); err != nil {
			e.logger.Error("Ошибка при обработке данных пользователя: %v", err)
			return nil, fmt.Errorf("ошибка обработки данных пользователя: %w", err)
		}
		users = append(users, user)
	}

	// Проверяем ошибки после итерации по результатам
	if err = rows.Err(); err != nil {
		e.logger.Error("Ошибка после итерации по пользователям: %v", err)
		return nil, fmt.Errorf("ошибка после итерации по пользователям: %w", err)
	}

	e.logger.Debug("Извлечено %d пользователей", len(users))
	return users, nil
}

// GetLastUserUpdateTime получает время последнего обновления пользователей
func (e *UserExtractor) GetLastUserUpdateTime() (time.Time, error) {
	var lastUpdateTime time.Time

	// Получаем время последнего обновления из таблицы пользователей
	err := e.db.QueryRow("SELECT MAX(created_at) FROM users").Scan(&lastUpdateTime)
	if err != nil {
		if err == sql.ErrNoRows {
			// Если нет пользователей, возвращаем нулевое время
			return time.Time{}, nil
		}
		e.logger.Error("Ошибка при получении времени последнего обновления пользователей: %v", err)
		return time.Time{}, fmt.Errorf("ошибка получения времени последнего обновления: %w", err)
	}

	return lastUpdateTime, nil
}
