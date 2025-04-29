package transform

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/models"
	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// Transformer координирует процесс преобразования данных из OLTP в OLAP
type Transformer struct {
	oltpDB            *sql.DB
	olapDB            *sql.DB
	logger            *utils.ETLLogger
	timeDimProcessor  *TimeDimensionProcessor
	userDimProcessor  *UserDimensionProcessor
	messageFProcessor *MessageFactsProcessor
	chatFProcessor    *ChatFactsProcessor
	dailyFProcessor   *DailyActivityProcessor
	hourlyFProcessor  *HourlyActivityProcessor
}

// NewTransformer создает новый экземпляр Transformer
func NewTransformer(oltpDB, olapDB *sql.DB, logger *utils.ETLLogger) *Transformer {
	return &Transformer{
		oltpDB:            oltpDB,
		olapDB:            olapDB,
		logger:            logger,
		timeDimProcessor:  NewTimeDimensionProcessor(oltpDB, olapDB, logger),
		userDimProcessor:  NewUserDimensionProcessor(oltpDB, olapDB, logger),
		messageFProcessor: NewMessageFactsProcessor(oltpDB, olapDB, logger),
		chatFProcessor:    NewChatFactsProcessor(oltpDB, olapDB, logger),
		dailyFProcessor:   NewDailyActivityProcessor(oltpDB, olapDB, logger),
		hourlyFProcessor:  NewHourlyActivityProcessor(oltpDB, olapDB, logger),
	}
}

// Transform выполняет полный процесс преобразования данных из OLTP в OLAP
func (t *Transformer) Transform(extractedData *models.ExtractedData) (*models.TransformedData, error) {
	startTime := time.Now()
	t.logger.Info("Начало фазы Transform (Преобразование данных)")

	// Создаем структуру для хранения трансформированных данных
	transformedData := &models.TransformedData{}

	// 1. Проверка и обновление измерения времени (обычно не требуется, так как уже создано)
	t.logger.Info("Проверка измерения времени...")
	err := t.timeDimProcessor.EnsureTimeDimension()
	if err != nil {
		t.logger.Error("Ошибка при проверке измерения времени: %v", err)
		return nil, fmt.Errorf("ошибка при проверке измерения времени: %w", err)
	}

	// 2. Преобразование данных пользователей
	t.logger.Info("Преобразование данных пользователей...")
	userDimensions, err := t.userDimProcessor.ProcessUserDimension(extractedData.Users, extractedData.Messages)
	if err != nil {
		t.logger.Error("Ошибка при преобразовании данных пользователей: %v", err)
		return nil, fmt.Errorf("ошибка при преобразовании данных пользователей: %w", err)
	}
	transformedData.Users = userDimensions

	// 3. Преобразование данных сообщений
	t.logger.Info("Преобразование данных сообщений...")
	messageFacts, err := t.messageFProcessor.ProcessMessageFacts(extractedData.Messages, extractedData.Chats)
	if err != nil {
		t.logger.Error("Ошибка при преобразовании данных сообщений: %v", err)
		return nil, fmt.Errorf("ошибка при преобразовании данных сообщений: %w", err)
	}
	transformedData.Messages = messageFacts

	// 4. Преобразование данных чатов
	t.logger.Info("Преобразование данных чатов...")
	chatFacts, err := t.chatFProcessor.ProcessChatFacts(extractedData.Chats, extractedData.Messages)
	if err != nil {
		t.logger.Error("Ошибка при преобразовании данных чатов: %v", err)
		return nil, fmt.Errorf("ошибка при преобразовании данных чатов: %w", err)
	}
	transformedData.Chats = chatFacts

	// 5. Формирование ежедневных агрегатов активности
	t.logger.Info("Формирование ежедневных агрегатов активности...")
	dailyActivityFacts, err := t.dailyFProcessor.ProcessDailyActivity()
	if err != nil {
		t.logger.Error("Ошибка при формировании ежедневных агрегатов: %v", err)
		return nil, fmt.Errorf("ошибка при формировании ежедневных агрегатов: %w", err)
	}
	transformedData.DailyActivity = dailyActivityFacts

	// 6. Формирование почасовых агрегатов активности
	t.logger.Info("Формирование почасовых агрегатов активности...")
	hourlyActivityFacts, err := t.hourlyFProcessor.ProcessHourlyActivity()
	if err != nil {
		t.logger.Error("Ошибка при формировании почасовых агрегатов: %v", err)
		return nil, fmt.Errorf("ошибка при формировании почасовых агрегатов: %w", err)
	}
	transformedData.HourlyActivity = hourlyActivityFacts

	// Заполняем метаданные
	transformedData.Metadata = models.ETLMetadata{
		LastRunTimestamp:     time.Now(),
		MessagesProcessed:    len(extractedData.Messages),
		ChatsProcessed:       len(extractedData.Chats),
		TimeDimensionUpdated: true,
		UserDimensionUpdated: true,
	}

	// Устанавливаем последний обработанный ID сообщения
	for _, msg := range extractedData.Messages {
		if msg.ID > transformedData.Metadata.LastProcessedMessageID {
			transformedData.Metadata.LastProcessedMessageID = msg.ID
		}
	}

	duration := time.Since(startTime)
	t.logger.Info("Фаза Transform завершена. Длительность: %v", duration)

	return transformedData, nil
}
