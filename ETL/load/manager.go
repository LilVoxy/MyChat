package load

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/models"
	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// LoadManager отвечает за управление процессом загрузки данных в OLAP
type LoadManager struct {
	db     *sql.DB
	logger *utils.ETLLogger
	loader Loader
}

// NewLoadManager создает новый экземпляр LoadManager
func NewLoadManager(db *sql.DB, logger *utils.ETLLogger) *LoadManager {
	return &LoadManager{
		db:     db,
		logger: logger,
		loader: NewOLAPLoader(db, logger),
	}
}

// Load выполняет фазу загрузки данных ETL-процесса
// Принимает обработанные данные из фазы Transform
func (m *LoadManager) Load(transformedData *models.TransformedData) error {
	startTime := time.Now()
	m.logger.Info("Начало фазы Load (Загрузка данных)")

	// 1. Загружаем измерение пользователей
	if len(transformedData.Users) > 0 {
		m.logger.Info("Загрузка измерения пользователей...")
		if err := m.loader.LoadUserDimension(transformedData.Users); err != nil {
			m.logger.Error("Ошибка при загрузке измерения пользователей: %v", err)
			return fmt.Errorf("ошибка при загрузке измерения пользователей: %w", err)
		}
	}

	// 2. Загружаем факты сообщений
	if len(transformedData.Messages) > 0 {
		m.logger.Info("Загрузка фактов сообщений...")
		if err := m.loader.LoadMessageFacts(transformedData.Messages); err != nil {
			m.logger.Error("Ошибка при загрузке фактов сообщений: %v", err)
			return fmt.Errorf("ошибка при загрузке фактов сообщений: %w", err)
		}
	}

	// 3. Загружаем факты чатов
	if len(transformedData.Chats) > 0 {
		m.logger.Info("Загрузка фактов чатов...")
		if err := m.loader.LoadChatFacts(transformedData.Chats); err != nil {
			m.logger.Error("Ошибка при загрузке фактов чатов: %v", err)
			return fmt.Errorf("ошибка при загрузке фактов чатов: %w", err)
		}
	}

	// 4. Загружаем ежедневные факты активности
	if len(transformedData.DailyActivity) > 0 {
		m.logger.Info("Загрузка фактов ежедневной активности...")
		if err := m.loader.LoadDailyActivityFacts(transformedData.DailyActivity); err != nil {
			m.logger.Error("Ошибка при загрузке фактов ежедневной активности: %v", err)
			return fmt.Errorf("ошибка при загрузке фактов ежедневной активности: %w", err)
		}
	}

	duration := time.Since(startTime)
	m.logger.Info("Фаза Load завершена. Длительность: %v", duration)

	return nil
}

// UpdateETLRunLog обновляет журнал запусков ETL
func (m *LoadManager) UpdateETLRunLog(runLog *models.ETLRunLog) error {
	// Подготавливаем запрос для обновления журнала запусков
	stmt, err := m.db.Prepare(`
		UPDATE etl_runs
		SET 
			end_time = ?,
			status = ?,
			users_processed = ?,
			chats_processed = ?,
			messages_processed = ?,
			last_processed_message_id = ?,
			error_message = ?,
			execution_time_seconds = ?
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("ошибка при подготовке запроса для обновления журнала: %w", err)
	}
	defer stmt.Close()

	// Рассчитываем время выполнения в секундах
	executionTime := runLog.EndTime.Sub(runLog.StartTime).Seconds()

	// Выполняем запрос
	_, err = stmt.Exec(
		runLog.EndTime,
		runLog.Status,
		runLog.UsersProcessed,
		runLog.ChatsProcessed,
		runLog.MessagesProcessed,
		runLog.LastProcessedMessageID,
		runLog.ErrorMessage,
		executionTime,
		runLog.ID,
	)
	if err != nil {
		return fmt.Errorf("ошибка при обновлении журнала запусков: %w", err)
	}

	return nil
}
