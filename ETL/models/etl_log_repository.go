package models

import (
	"database/sql"
	"fmt"
	"time"
)

// MySQLETLLogRepository реализация ETLLogRepository для MySQL
type MySQLETLLogRepository struct {
	db *sql.DB
}

// NewMySQLETLLogRepository создает новый экземпляр MySQLETLLogRepository
func NewMySQLETLLogRepository(db *sql.DB) *MySQLETLLogRepository {
	return &MySQLETLLogRepository{
		db: db,
	}
}

// CreateETLLogTable создает таблицу для логирования ETL процесса, если она не существует
func (r *MySQLETLLogRepository) CreateETLLogTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS chat_analytics.etl_run_log (
		id INT AUTO_INCREMENT PRIMARY KEY,
		start_time TIMESTAMP NOT NULL,
		end_time TIMESTAMP NULL,
		status ENUM('success', 'failed', 'in_progress') NOT NULL DEFAULT 'in_progress',
		users_processed INT DEFAULT 0,
		chats_processed INT DEFAULT 0,
		messages_processed INT DEFAULT 0,
		last_processed_message_id INT DEFAULT 0,
		error_message TEXT,
		execution_time_seconds FLOAT
	);
	`

	_, err := r.db.Exec(query)
	if err != nil {
		return fmt.Errorf("ошибка при создании таблицы etl_run_log: %w", err)
	}

	return nil
}

// CreateLogEntry создает новую запись о запуске ETL
func (r *MySQLETLLogRepository) CreateLogEntry(startTime time.Time) (int, error) {
	query := `
	INSERT INTO chat_analytics.etl_run_log (start_time, status) 
	VALUES (?, 'in_progress')
	`

	result, err := r.db.Exec(query, startTime)
	if err != nil {
		return 0, fmt.Errorf("ошибка при создании записи о запуске ETL: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ошибка при получении ID созданной записи: %w", err)
	}

	return int(id), nil
}

// UpdateLogEntrySuccess обновляет запись при успешном завершении ETL
func (r *MySQLETLLogRepository) UpdateLogEntrySuccess(
	id int,
	endTime time.Time,
	usersProcessed,
	chatsProcessed,
	messagesProcessed,
	lastProcessedMessageID int) error {

	// Рассчитываем время выполнения в секундах
	var startTime time.Time
	err := r.db.QueryRow("SELECT start_time FROM chat_analytics.etl_run_log WHERE id = ?", id).Scan(&startTime)
	if err != nil {
		return fmt.Errorf("ошибка при получении времени начала ETL: %w", err)
	}

	executionTime := endTime.Sub(startTime).Seconds()

	// Обновляем запись
	query := `
	UPDATE chat_analytics.etl_run_log 
	SET 
		end_time = ?,
		status = 'success',
		users_processed = ?,
		chats_processed = ?,
		messages_processed = ?,
		last_processed_message_id = ?,
		execution_time_seconds = ?
	WHERE id = ?
	`

	_, err = r.db.Exec(
		query,
		endTime,
		usersProcessed,
		chatsProcessed,
		messagesProcessed,
		lastProcessedMessageID,
		executionTime,
		id,
	)

	if err != nil {
		return fmt.Errorf("ошибка при обновлении записи о запуске ETL: %w", err)
	}

	return nil
}

// UpdateLogEntryFailure обновляет запись при неудачном завершении ETL
func (r *MySQLETLLogRepository) UpdateLogEntryFailure(id int, endTime time.Time, errorMessage string) error {
	// Рассчитываем время выполнения в секундах
	var startTime time.Time
	err := r.db.QueryRow("SELECT start_time FROM chat_analytics.etl_run_log WHERE id = ?", id).Scan(&startTime)
	if err != nil {
		return fmt.Errorf("ошибка при получении времени начала ETL: %w", err)
	}

	executionTime := endTime.Sub(startTime).Seconds()

	// Обновляем запись
	query := `
	UPDATE chat_analytics.etl_run_log 
	SET 
		end_time = ?,
		status = 'failed',
		error_message = ?,
		execution_time_seconds = ?
	WHERE id = ?
	`

	_, err = r.db.Exec(query, endTime, errorMessage, executionTime, id)
	if err != nil {
		return fmt.Errorf("ошибка при обновлении записи о запуске ETL: %w", err)
	}

	return nil
}

// GetLastSuccessfulRun получает информацию о последнем успешном запуске ETL
func (r *MySQLETLLogRepository) GetLastSuccessfulRun() (*ETLRunLog, error) {
	query := `
	SELECT 
		id, start_time, end_time, status, 
		users_processed, chats_processed, messages_processed, 
		last_processed_message_id, IFNULL(error_message, ''), execution_time_seconds
	FROM chat_analytics.etl_run_log 
	WHERE status = 'success' 
	ORDER BY end_time DESC 
	LIMIT 1
	`

	var log ETLRunLog
	err := r.db.QueryRow(query).Scan(
		&log.ID, &log.StartTime, &log.EndTime, &log.Status,
		&log.UsersProcessed, &log.ChatsProcessed, &log.MessagesProcessed,
		&log.LastProcessedMessageID, &log.ErrorMessage, &log.ExecutionTimeSeconds,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Нет успешных запусков
		}
		return nil, fmt.Errorf("ошибка при получении информации о последнем успешном запуске ETL: %w", err)
	}

	return &log, nil
}

// GetETLRunStats получает статистику о запусках ETL за определенный период
func (r *MySQLETLLogRepository) GetETLRunStats(days int) ([]ETLRunLog, error) {
	query := `
	SELECT 
		id, start_time, end_time, status, 
		users_processed, chats_processed, messages_processed, 
		last_processed_message_id, IFNULL(error_message, ''), execution_time_seconds
	FROM chat_analytics.etl_run_log 
	WHERE start_time >= DATE_SUB(NOW(), INTERVAL ? DAY)
	ORDER BY start_time DESC
	`

	rows, err := r.db.Query(query, days)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении статистики запусков ETL: %w", err)
	}
	defer rows.Close()

	var logs []ETLRunLog
	for rows.Next() {
		var log ETLRunLog
		err := rows.Scan(
			&log.ID, &log.StartTime, &log.EndTime, &log.Status,
			&log.UsersProcessed, &log.ChatsProcessed, &log.MessagesProcessed,
			&log.LastProcessedMessageID, &log.ErrorMessage, &log.ExecutionTimeSeconds,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка при сканировании записи о запуске ETL: %w", err)
		}
		logs = append(logs, log)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка после итерации по записям о запусках ETL: %w", err)
	}

	return logs, nil
}

// GetETLStateMonitor получает информацию о текущем состоянии ETL процесса
func (r *MySQLETLLogRepository) GetETLStateMonitor() (*ETLStateMonitor, error) {
	// Получаем последний успешный запуск
	lastSuccessful, err := r.GetLastSuccessfulRun()
	if err != nil {
		return nil, err
	}

	// Получаем последний неудачный запуск
	var lastFailed *ETLRunLog
	query := `
	SELECT 
		id, start_time, end_time, status, 
		users_processed, chats_processed, messages_processed, 
		last_processed_message_id, IFNULL(error_message, ''), execution_time_seconds
	FROM chat_analytics.etl_run_log 
	WHERE status = 'failed' 
	ORDER BY end_time DESC 
	LIMIT 1
	`

	row := r.db.QueryRow(query)
	var log ETLRunLog
	err = row.Scan(
		&log.ID, &log.StartTime, &log.EndTime, &log.Status,
		&log.UsersProcessed, &log.ChatsProcessed, &log.MessagesProcessed,
		&log.LastProcessedMessageID, &log.ErrorMessage, &log.ExecutionTimeSeconds,
	)

	if err == nil {
		lastFailed = &log
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("ошибка при получении информации о последнем неудачном запуске ETL: %w", err)
	}

	// Получаем текущий запуск (если есть)
	var currentRun *ETLRunLog
	query = `
	SELECT 
		id, start_time, IFNULL(end_time, NOW()) as end_time, status, 
		users_processed, chats_processed, messages_processed, 
		last_processed_message_id, IFNULL(error_message, ''), 
		TIMESTAMPDIFF(SECOND, start_time, NOW()) as execution_time_seconds
	FROM chat_analytics.etl_run_log 
	WHERE status = 'in_progress' 
	ORDER BY start_time DESC 
	LIMIT 1
	`

	row = r.db.QueryRow(query)
	err = row.Scan(
		&log.ID, &log.StartTime, &log.EndTime, &log.Status,
		&log.UsersProcessed, &log.ChatsProcessed, &log.MessagesProcessed,
		&log.LastProcessedMessageID, &log.ErrorMessage, &log.ExecutionTimeSeconds,
	)

	if err == nil {
		currentRun = &log
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("ошибка при получении информации о текущем запуске ETL: %w", err)
	}

	// Получаем общую статистику запусков
	var totalSuccess, totalFailed int
	var avgExecutionTime float64
	var totalItems int

	err = r.db.QueryRow(`
		SELECT 
			COUNT(*) FILTER (WHERE status = 'success'),
			COUNT(*) FILTER (WHERE status = 'failed'),
			AVG(execution_time_seconds) FILTER (WHERE status = 'success'),
			SUM(users_processed + chats_processed + messages_processed) FILTER (WHERE status = 'success')
		FROM chat_analytics.etl_run_log
	`).Scan(&totalSuccess, &totalFailed, &avgExecutionTime, &totalItems)

	if err != nil {
		// Если MySQL не поддерживает FILTER, используем альтернативный запрос
		err = r.db.QueryRow(`
			SELECT 
				SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END),
				SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END),
				AVG(CASE WHEN status = 'success' THEN execution_time_seconds ELSE NULL END),
				SUM(CASE WHEN status = 'success' THEN users_processed + chats_processed + messages_processed ELSE 0 END)
			FROM chat_analytics.etl_run_log
		`).Scan(&totalSuccess, &totalFailed, &avgExecutionTime, &totalItems)

		if err != nil {
			return nil, fmt.Errorf("ошибка при получении статистики запусков ETL: %w", err)
		}
	}

	return &ETLStateMonitor{
		LastSuccessfulRun:       lastSuccessful,
		LastFailedRun:           lastFailed,
		CurrentRun:              currentRun,
		TotalSuccessfulRuns:     totalSuccess,
		TotalFailedRuns:         totalFailed,
		AvgExecutionTimeSeconds: avgExecutionTime,
		TotalItemsProcessed:     totalItems,
	}, nil
}
