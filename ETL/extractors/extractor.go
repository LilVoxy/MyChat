package extractors

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/models"
	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// Extractor координирует процесс извлечения данных из OLTP
type Extractor struct {
	db               *sql.DB
	logger           *utils.ETLLogger
	userExtractor    *UserExtractor
	chatExtractor    *ChatExtractor
	messageExtractor *MessageExtractor
	batchSize        int
}

// NewExtractor создает новый экземпляр Extractor
func NewExtractor(db *sql.DB, logger *utils.ETLLogger, batchSize int) *Extractor {
	return &Extractor{
		db:               db,
		logger:           logger,
		userExtractor:    NewUserExtractor(db, logger),
		chatExtractor:    NewChatExtractor(db, logger),
		messageExtractor: NewMessageExtractor(db, logger),
		batchSize:        batchSize,
	}
}

// Extract выполняет извлечение данных из OLTP для ETL процесса
// lastRunTime - время последнего запуска ETL, для инкрементального извлечения данных
// lastProcessedMessageID - ID последнего обработанного сообщения
func (e *Extractor) Extract(lastRunTime time.Time, lastProcessedMessageID int) (*models.ExtractedData, error) {
	startTime := time.Now()
	e.logger.LogExtractStart()

	var extractedData models.ExtractedData
	var err error

	// Извлекаем пользователей
	extractedData.Users, err = e.userExtractor.ExtractUsers(lastRunTime, e.batchSize)
	if err != nil {
		e.logger.Error("Ошибка при извлечении пользователей: %v", err)
		return nil, fmt.Errorf("ошибка извлечения пользователей: %w", err)
	}

	// Извлекаем чаты
	extractedData.Chats, err = e.chatExtractor.ExtractChats(lastRunTime, e.batchSize)
	if err != nil {
		e.logger.Error("Ошибка при извлечении чатов: %v", err)
		return nil, fmt.Errorf("ошибка извлечения чатов: %w", err)
	}

	// Извлекаем сообщения
	extractedData.Messages, err = e.messageExtractor.ExtractMessages(time.Time{}, lastProcessedMessageID, e.batchSize)
	if err != nil {
		e.logger.Error("Ошибка при извлечении сообщений: %v", err)
		return nil, fmt.Errorf("ошибка извлечения сообщений: %w", err)
	}

	// Дополнительно извлекаем чаты, связанные с извлеченными сообщениями, но не попавшие в выборку BatchSize
	chatIDs := make(map[int]bool)
	existingChatIDs := make(map[int]bool)

	// Создаем карту существующих чатов
	for _, chat := range extractedData.Chats {
		existingChatIDs[chat.ID] = true
	}

	// Собираем ID всех чатов из сообщений
	for _, msg := range extractedData.Messages {
		chatIDs[msg.ChatID] = true
	}

	// Находим ID чатов, которые есть в сообщениях, но отсутствуют в извлеченных чатах
	var missingChatIDs []int
	for chatID := range chatIDs {
		if !existingChatIDs[chatID] {
			missingChatIDs = append(missingChatIDs, chatID)
		}
	}

	// Если есть отсутствующие чаты, извлекаем их
	if len(missingChatIDs) > 0 {
		e.logger.Debug("Найдено %d отсутствующих чатов, связанных с сообщениями. Извлекаем дополнительно.", len(missingChatIDs))

		chatMap, err := e.chatExtractor.GetChatsByIDs(missingChatIDs)
		if err != nil {
			e.logger.Error("Ошибка при извлечении дополнительных чатов: %v", err)
			// Продолжаем выполнение, т.к. это некритичная ошибка
		} else {
			// Добавляем найденные чаты к основному списку
			for _, chat := range chatMap {
				extractedData.Chats = append(extractedData.Chats, chat)
			}
			e.logger.Debug("Дополнительно извлечено %d чатов", len(chatMap))
		}
	}

	// Записываем время запуска
	extractedData.LastRunTS = time.Now()

	// Выводим информацию о завершении
	e.logger.LogExtractComplete(
		len(extractedData.Messages),
		len(extractedData.Users),
		len(extractedData.Chats),
		time.Since(startTime),
	)

	return &extractedData, nil
}

// GetETLMetadata получает метаданные для ETL
func (e *Extractor) GetETLMetadata() (models.ETLMetadata, error) {
	var metadata models.ETLMetadata
	var err error

	// Получаем последнее время обновления сообщений
	metadata.LastRunTimestamp, err = e.messageExtractor.GetLastMessageUpdateTime()
	if err != nil {
		e.logger.Error("Ошибка при получении времени последнего обновления сообщений: %v", err)
		return metadata, err
	}

	// Получаем ID последнего сообщения
	metadata.LastProcessedMessageID, err = e.messageExtractor.GetLastMessageID()
	if err != nil {
		e.logger.Error("Ошибка при получении ID последнего сообщения: %v", err)
		return metadata, err
	}

	// Дополнительно можно получить ID последнего чата, но это не обязательно

	return metadata, nil
}

// GetMessagesByChat получает все сообщения для указанного чата
func (e *Extractor) GetMessagesByChat(chatID int) ([]models.MessageOLTP, error) {
	return e.messageExtractor.GetMessagesByChatID(chatID)
}

// GetChatInfo получает информацию о чате по ID
func (e *Extractor) GetChatInfo(chatID int) (*models.ChatOLTP, error) {
	chatMap, err := e.chatExtractor.GetChatsByIDs([]int{chatID})
	if err != nil {
		return nil, err
	}

	chat, exists := chatMap[chatID]
	if !exists {
		return nil, fmt.Errorf("чат с ID %d не найден", chatID)
	}

	return &chat, nil
}
