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

// ChatFactsProcessor отвечает за преобразование данных чатов
type ChatFactsProcessor struct {
	oltpDB *sql.DB
	olapDB *sql.DB
	logger *utils.ETLLogger
}

// NewChatFactsProcessor создает новый экземпляр ChatFactsProcessor
func NewChatFactsProcessor(oltpDB, olapDB *sql.DB, logger *utils.ETLLogger) *ChatFactsProcessor {
	return &ChatFactsProcessor{
		oltpDB: oltpDB,
		olapDB: olapDB,
		logger: logger,
	}
}

// ProcessChatFacts обрабатывает данные чатов и возвращает трансформированные факты
func (p *ChatFactsProcessor) ProcessChatFacts(chats []models.ChatOLTP, messages []models.MessageOLTP) ([]models.ChatFact, error) {
	p.logger.Debug("Обработка фактов чатов...")

	if len(chats) == 0 {
		p.logger.Debug("Нет данных чатов для обработки")
		return []models.ChatFact{}, nil
	}

	// Результирующий список трансформированных фактов чатов
	transformedChats := make([]models.ChatFact, 0, len(chats))

	// Группируем сообщения по чатам
	chatMessages := make(map[int][]models.MessageOLTP)
	for _, msg := range messages {
		chatMessages[msg.ChatID] = append(chatMessages[msg.ChatID], msg)
	}

	// Сортируем сообщения в каждом чате по времени
	for chatID, msgs := range chatMessages {
		sort.Slice(msgs, func(i, j int) bool {
			return msgs[i].CreatedAt.Before(msgs[j].CreatedAt)
		})
		chatMessages[chatID] = msgs
	}

	// Получаем маппинг time_id через специальную функцию
	timeIDMap, err := p.getTimeIDMapping()
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении маппинга time_id: %w", err)
	}

	// Обрабатываем каждый чат
	for _, chat := range chats {
		// Получаем сообщения для данного чата
		msgs, hasMsgs := chatMessages[chat.ID]

		// Определяем start_time_id и end_time_id
		var startTimeID, endTimeID int
		var totalMessages, buyerMessages, sellerMessages int
		var totalLength int
		var responseTimes []float64
		var chatDurationHours float64

		if !hasMsgs || len(msgs) == 0 {
			// Если в чате нет сообщений, используем время создания чата для обоих time_id
			dateKey := chat.CreatedAt.Format("2006-01-02")
			hourKey := chat.CreatedAt.Hour()
			startTimeID = p.getTimeID(timeIDMap, dateKey, hourKey)

			if startTimeID == 0 {
				// Если не найден ID, используем временный ID
				startTimeID = -1 * (int(chat.CreatedAt.Unix()) % 10000)
				p.logger.Debug("Использован временный time_id для чата %d: %d", chat.ID, startTimeID)
			}

			endTimeID = startTimeID
			chatDurationHours = 0
		} else {
			// Если в чате есть сообщения
			firstMsg := msgs[0]
			lastMsg := msgs[len(msgs)-1]

			// Получаем время первого сообщения
			firstDateKey := firstMsg.CreatedAt.Format("2006-01-02")
			firstHourKey := firstMsg.CreatedAt.Hour()
			startTimeID = p.getTimeID(timeIDMap, firstDateKey, firstHourKey)

			if startTimeID == 0 {
				// Если не найден ID, используем временный ID
				startTimeID = -1 * (int(firstMsg.CreatedAt.Unix()) % 10000)
				p.logger.Debug("Использован временный start_time_id для чата %d: %d", chat.ID, startTimeID)
			}

			// Получаем время последнего сообщения
			lastDateKey := lastMsg.CreatedAt.Format("2006-01-02")
			lastHourKey := lastMsg.CreatedAt.Hour()
			endTimeID = p.getTimeID(timeIDMap, lastDateKey, lastHourKey)

			if endTimeID == 0 {
				// Если не найден ID, используем временный ID
				endTimeID = -1 * (int(lastMsg.CreatedAt.Unix()) % 10000)
				p.logger.Debug("Использован временный end_time_id для чата %d: %d", chat.ID, endTimeID)
			}

			// Рассчитываем длительность чата в часах
			if len(msgs) <= 1 {
				chatDurationHours = 0
			} else {
				chatDurationHours = lastMsg.CreatedAt.Sub(firstMsg.CreatedAt).Hours()
			}

			// Рассчитываем метрики сообщений
			totalMessages = len(msgs)

			for i, msg := range msgs {
				// Подсчитываем сообщения по ролям
				if msg.SenderID == chat.BuyerID {
					buyerMessages++
				} else if msg.SenderID == chat.SellerID {
					sellerMessages++
				}

				// Суммируем длину сообщений
				totalLength += len(strings.TrimSpace(msg.Message))

				// Рассчитываем время ответа
				if i > 0 && msg.SenderID != msgs[i-1].SenderID {
					// Это ответное сообщение
					responseTime := msg.CreatedAt.Sub(msgs[i-1].CreatedAt).Minutes()
					responseTimes = append(responseTimes, responseTime)
				}
			}
		}

		// Рассчитываем среднюю длину сообщения
		var avgMessageLength float64
		if totalMessages > 0 {
			avgMessageLength = float64(totalLength) / float64(totalMessages)
		}

		// Рассчитываем среднее время ответа
		var avgResponseTime float64
		if len(responseTimes) > 0 {
			var sum float64
			for _, t := range responseTimes {
				sum += t
			}
			avgResponseTime = sum / float64(len(responseTimes))
		}

		// Создаем объект ChatFact
		chatFact := models.ChatFact{
			ChatID:                 chat.ID,
			StartTimeID:            startTimeID,
			EndTimeID:              endTimeID,
			BuyerID:                chat.BuyerID,
			SellerID:               chat.SellerID,
			TotalMessages:          totalMessages,
			BuyerMessages:          buyerMessages,
			SellerMessages:         sellerMessages,
			AvgMessageLength:       avgMessageLength,
			AvgResponseTimeMinutes: avgResponseTime,
			ChatDurationHours:      chatDurationHours,
		}

		// Добавляем факт в результирующий список
		transformedChats = append(transformedChats, chatFact)
	}

	p.logger.Info("Обработано фактов чатов: %d", len(transformedChats))
	return transformedChats, nil
}

// getTimeIDMapping получает маппинг дат и часов к time_id из OLAP базы данных
func (p *ChatFactsProcessor) getTimeIDMapping() (map[string]map[int]int, error) {
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
func (p *ChatFactsProcessor) getTimeID(timeIDMap map[string]map[int]int, date string, hour int) int {
	if hourMap, dateExists := timeIDMap[date]; dateExists {
		if timeID, hourExists := hourMap[hour]; hourExists {
			return timeID
		}
	}
	return 0 // Возвращаем 0, если не найдено
}
