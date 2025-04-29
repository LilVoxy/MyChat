package transform

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/models"
	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// MessageFactsProcessor отвечает за преобразование данных сообщений
type MessageFactsProcessor struct {
	oltpDB *sql.DB
	olapDB *sql.DB
	logger *utils.ETLLogger
}

// NewMessageFactsProcessor создает новый экземпляр MessageFactsProcessor
func NewMessageFactsProcessor(oltpDB, olapDB *sql.DB, logger *utils.ETLLogger) *MessageFactsProcessor {
	return &MessageFactsProcessor{
		oltpDB: oltpDB,
		olapDB: olapDB,
		logger: logger,
	}
}

// ProcessMessageFacts обрабатывает данные сообщений и возвращает трансформированные факты
func (p *MessageFactsProcessor) ProcessMessageFacts(messages []models.MessageOLTP, chats []models.ChatOLTP) ([]models.MessageFact, error) {
	p.logger.Debug("Обработка фактов сообщений...")

	if len(messages) == 0 {
		p.logger.Debug("Нет данных сообщений для обработки")
		return []models.MessageFact{}, nil
	}

	// Результирующий список трансформированных фактов сообщений
	transformedMessages := make([]models.MessageFact, 0, len(messages))

	// Создаем карту чатов для быстрого доступа
	chatMap := make(map[int]models.ChatOLTP)
	for _, chat := range chats {
		chatMap[chat.ID] = chat
	}

	// Группируем сообщения по чатам для определения первых сообщений и расчета времени ответа
	chatMessages := make(map[int][]models.MessageOLTP)
	for _, msg := range messages {
		chatMessages[msg.ChatID] = append(chatMessages[msg.ChatID], msg)
	}

	// Для каждого чата сортируем сообщения по времени и определяем первые сообщения
	for chatID, msgs := range chatMessages {
		// Сортируем сообщения по времени (предполагаем, что они уже отсортированы)
		sort.Slice(msgs, func(i, j int) bool {
			return msgs[i].CreatedAt.Before(msgs[j].CreatedAt)
		})
		chatMessages[chatID] = msgs
	}

	// Получаем соответствие дат и часов к time_id через специальную функцию
	timeIDMap, err := p.getTimeIDMapping()
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении маппинга time_id: %w", err)
	}

	// Обрабатываем каждое сообщение
	for chatID, msgs := range chatMessages {
		// Получаем информацию о чате
		chat, chatExists := chatMap[chatID]
		if !chatExists {
			p.logger.Debug("Чат с ID %d не найден в извлеченных данных", chatID)
			continue
		}

		for i, msg := range msgs {
			// Определяем, является ли сообщение первым в чате
			isFirst := (i == 0)

			// Определяем получателя сообщения
			var recipientID int
			if msg.SenderID == chat.BuyerID {
				recipientID = chat.SellerID
			} else {
				recipientID = chat.BuyerID
			}

			// Рассчитываем длину сообщения
			messageLength := len(strings.TrimSpace(msg.Message))

			// Рассчитываем время ответа
			var responseTime float64
			if i > 0 && msg.SenderID != msgs[i-1].SenderID {
				// Это ответное сообщение, рассчитываем время в минутах
				responseTime = msg.CreatedAt.Sub(msgs[i-1].CreatedAt).Minutes()
			} else {
				// Не ответное сообщение (первое или от того же отправителя)
				responseTime = 0
			}

			// Получаем time_id из маппинга
			dateKey := msg.CreatedAt.Format("2006-01-02")
			hourKey := msg.CreatedAt.Hour()
			timeID := p.getTimeID(timeIDMap, dateKey, hourKey)

			// Если не удалось найти time_id, создаем временный ID
			if timeID == 0 {
				// В реальной системе здесь можно было бы создать новую запись в time_dimension
				// Для упрощения просто используем временный ID
				timeID = -1 * (int(msg.CreatedAt.Unix()) % 10000) // Отрицательный ID для индикации временного значения
				p.logger.Debug("Использован временный time_id для сообщения %d: %d", msg.ID, timeID)
			}

			// Создаем объект MessageFact
			messageFact := models.MessageFact{
				MessageID:           msg.ID,
				TimeID:              timeID,
				SenderID:            msg.SenderID,
				RecipientID:         recipientID,
				ChatID:              msg.ChatID,
				MessageLength:       messageLength,
				ResponseTimeMinutes: responseTime,
				IsFirstInChat:       isFirst,
			}

			// Добавляем факт в результирующий список
			transformedMessages = append(transformedMessages, messageFact)
		}
	}

	p.logger.Info("Обработано фактов сообщений: %d", len(transformedMessages))
	return transformedMessages, nil
}

// getTimeIDMapping получает маппинг дат и часов к time_id из OLAP базы данных
func (p *MessageFactsProcessor) getTimeIDMapping() (map[string]map[int]int, error) {
	timeIDMap := make(map[string]map[int]int)

	// В реальной имплементации здесь был бы запрос к OLAP базе для получения маппинга
	// Для прототипа создаем тестовые данные

	// Создаем данные для последних 30 дней (для примера)
	now := time.Now()
	for i := 0; i < 30; i++ {
		date := now.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		timeIDMap[dateStr] = make(map[int]int)

		// Для каждого часа в дне
		for hour := 0; hour < 24; hour++ {
			// Генерируем простой ID на основе даты и часа
			// В реальной системе ID были бы получены из базы данных
			timeIDMap[dateStr][hour] = 1000 + i*100 + hour
		}
	}

	return timeIDMap, nil
}

// getTimeID возвращает time_id для указанной даты и часа из маппинга
func (p *MessageFactsProcessor) getTimeID(timeIDMap map[string]map[int]int, date string, hour int) int {
	if hourMap, dateExists := timeIDMap[date]; dateExists {
		if timeID, hourExists := hourMap[hour]; hourExists {
			return timeID
		}
	}
	return 0 // Возвращаем 0, если не найдено
}
