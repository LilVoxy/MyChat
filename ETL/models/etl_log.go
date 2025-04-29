package models

import (
	"time"
)

// ETLRunLog представляет запись о запуске ETL процесса
type ETLRunLog struct {
	ID                     int       `json:"id"`
	StartTime              time.Time `json:"start_time"`
	EndTime                time.Time `json:"end_time"`
	Status                 string    `json:"status"` // "success", "failed", "in_progress"
	UsersProcessed         int       `json:"users_processed"`
	ChatsProcessed         int       `json:"chats_processed"`
	MessagesProcessed      int       `json:"messages_processed"`
	LastProcessedMessageID int       `json:"last_processed_message_id"`
	ErrorMessage           string    `json:"error_message,omitempty"`
	ExecutionTimeSeconds   float64   `json:"execution_time_seconds"`
}

// ETLLogRepository представляет репозиторий для работы с логами ETL
type ETLLogRepository interface {
	// CreateLogEntry создает новую запись о запуске ETL
	CreateLogEntry(startTime time.Time) (int, error)

	// UpdateLogEntrySuccess обновляет запись при успешном завершении ETL
	UpdateLogEntrySuccess(
		id int,
		endTime time.Time,
		usersProcessed,
		chatsProcessed,
		messagesProcessed,
		lastProcessedMessageID int) error

	// UpdateLogEntryFailure обновляет запись при неудачном завершении ETL
	UpdateLogEntryFailure(id int, endTime time.Time, errorMessage string) error

	// GetLastSuccessfulRun получает информацию о последнем успешном запуске ETL
	GetLastSuccessfulRun() (*ETLRunLog, error)

	// GetETLRunStats получает статистику о запусках ETL за определенный период
	GetETLRunStats(days int) ([]ETLRunLog, error)
}

// ETLStateMonitor предоставляет информацию о текущем состоянии ETL процесса
type ETLStateMonitor struct {
	LastSuccessfulRun       *ETLRunLog `json:"last_successful_run"`
	LastFailedRun           *ETLRunLog `json:"last_failed_run,omitempty"`
	CurrentRun              *ETLRunLog `json:"current_run,omitempty"`
	TotalSuccessfulRuns     int        `json:"total_successful_runs"`
	TotalFailedRuns         int        `json:"total_failed_runs"`
	AvgExecutionTimeSeconds float64    `json:"avg_execution_time_seconds"`
	TotalItemsProcessed     int        `json:"total_items_processed"` // Общее количество обработанных объектов (сообщения + чаты + пользователи)
}
