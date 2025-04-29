package load

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/models"
	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// ETLExample показывает пример использования Loader в ETL-процессе
func ETLExample(oltpDB, olapDB *sql.DB, logger *utils.ETLLogger) error {
	// 1. Создаем загрузчик
	loader := NewOLAPLoader(olapDB, logger)

	// 2. Предположим, что уже выполнена трансформация и получены следующие данные:
	// (это пример данных, в реальном коде данные будут получены из фазы Transform)

	// 2.1 Данные пользователей
	userDimensions := []models.UserDimension{
		{
			ID:                     1,
			RegistrationDate:       time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
			DaysActive:             30,
			TotalChats:             5,
			TotalMessages:          20,
			AvgResponseTimeMinutes: 2.5,
			ActivityLevel:          "medium",
		},
		// ... другие пользователи
	}

	// 2.2 Факты сообщений
	messageFacts := []models.MessageFact{
		{
			MessageID:           101,
			TimeID:              1001,
			SenderID:            1,
			RecipientID:         2,
			ChatID:              501,
			MessageLength:       15,
			ResponseTimeMinutes: 1.2,
			IsFirstInChat:       true,
		},
		// ... другие сообщения
	}

	// 2.3 Факты чатов
	chatFacts := []models.ChatFact{
		{
			ChatID:                 501,
			StartTimeID:            1001,
			EndTimeID:              1005,
			BuyerID:                1,
			SellerID:               2,
			TotalMessages:          10,
			BuyerMessages:          6,
			SellerMessages:         4,
			AvgMessageLength:       12.5,
			AvgResponseTimeMinutes: 1.8,
			ChatDurationHours:      2.3,
		},
		// ... другие чаты
	}

	// 2.4 Ежедневные факты активности
	dailyFacts := []models.DailyActivityFact{
		{
			DateID:                 1001,
			TotalMessages:          100,
			TotalNewChats:          15,
			ActiveUsers:            25,
			NewUsers:               5,
			AvgMessagesPerChat:     6.7,
			AvgResponseTimeMinutes: 2.1,
			PeakHour:               14,
			PeakHourMessages:       20,
		},
		// ... другие дни
	}

	// 2.5 Почасовые факты активности
	hourlyFacts := []models.HourlyActivityFact{
		{
			DateID:                 1001,
			HourOfDay:              14,
			TotalMessages:          20,
			TotalNewChats:          3,
			ActiveUsers:            8,
			AvgResponseTimeMinutes: 1.9,
		},
		// ... другие часы
	}

	// 3. Загружаем данные в OLAP
	logger.Info("Начало ETL-процесса (фаза Load)...")

	// 3.1 Загружаем данные пользователей
	logger.Info("Загрузка измерения пользователей...")
	if err := loader.LoadUserDimension(userDimensions); err != nil {
		return fmt.Errorf("ошибка при загрузке пользователей: %w", err)
	}

	// 3.2 Загружаем факты сообщений
	logger.Info("Загрузка фактов сообщений...")
	if err := loader.LoadMessageFacts(messageFacts); err != nil {
		return fmt.Errorf("ошибка при загрузке сообщений: %w", err)
	}

	// 3.3 Загружаем факты чатов
	logger.Info("Загрузка фактов чатов...")
	if err := loader.LoadChatFacts(chatFacts); err != nil {
		return fmt.Errorf("ошибка при загрузке чатов: %w", err)
	}

	// 3.4 Загружаем ежедневные факты активности
	logger.Info("Загрузка фактов ежедневной активности...")
	if err := loader.LoadDailyActivityFacts(dailyFacts); err != nil {
		return fmt.Errorf("ошибка при загрузке ежедневной активности: %w", err)
	}

	// 3.5 Загружаем почасовые факты активности
	logger.Info("Загрузка фактов почасовой активности...")
	if err := loader.LoadHourlyActivityFacts(hourlyFacts); err != nil {
		return fmt.Errorf("ошибка при загрузке почасовой активности: %w", err)
	}

	logger.Info("ETL-процесс (фаза Load) успешно завершен")
	return nil
}
