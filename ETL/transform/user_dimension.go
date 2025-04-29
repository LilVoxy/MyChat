package transform

import (
	"database/sql"
	"math"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/models"
	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// UserDimensionProcessor отвечает за преобразование данных пользователей
type UserDimensionProcessor struct {
	oltpDB *sql.DB
	olapDB *sql.DB
	logger *utils.ETLLogger
}

// NewUserDimensionProcessor создает новый экземпляр UserDimensionProcessor
func NewUserDimensionProcessor(oltpDB, olapDB *sql.DB, logger *utils.ETLLogger) *UserDimensionProcessor {
	return &UserDimensionProcessor{
		oltpDB: oltpDB,
		olapDB: olapDB,
		logger: logger,
	}
}

// ProcessUserDimension обрабатывает данные пользователей и возвращает трансформированные данные
func (p *UserDimensionProcessor) ProcessUserDimension(users []models.UserOLTP, messages []models.MessageOLTP) ([]models.UserDimension, error) {
	p.logger.Debug("Обработка измерения пользователей...")

	if len(users) == 0 {
		p.logger.Debug("Нет данных пользователей для обработки")
		return []models.UserDimension{}, nil
	}

	// Результирующий список трансформированных данных пользователей
	transformedUsers := make([]models.UserDimension, 0, len(users))

	// Подготавливаем карту последней активности пользователей
	lastActivityMap := make(map[int]time.Time)
	totalMessagesMap := make(map[int]int)
	responseTimes := make(map[int][]float64) // для расчета среднего времени ответа

	// Для каждого пользователя находим его сообщения
	for _, msg := range messages {
		// Обновляем время последней активности
		senderID := msg.SenderID
		if lastActivity, exists := lastActivityMap[senderID]; !exists || msg.CreatedAt.After(lastActivity) {
			lastActivityMap[senderID] = msg.CreatedAt
		}

		// Подсчитываем общее количество сообщений
		totalMessagesMap[senderID]++
	}

	// Рассчитываем времена ответов
	chatMessages := make(map[int][]models.MessageOLTP)
	for _, msg := range messages {
		chatMessages[msg.ChatID] = append(chatMessages[msg.ChatID], msg)
	}

	// Для каждого чата анализируем времена ответов
	for _, msgs := range chatMessages {
		// Сортируем сообщения по времени (предполагаем, что они уже отсортированы)
		if len(msgs) < 2 {
			continue
		}

		var prevMsg models.MessageOLTP
		for i, msg := range msgs {
			if i == 0 {
				prevMsg = msg
				continue
			}

			// Если отправитель текущего сообщения не совпадает с отправителем предыдущего,
			// то это ответ, и мы можем рассчитать время ответа
			if msg.SenderID != prevMsg.SenderID {
				responseTime := msg.CreatedAt.Sub(prevMsg.CreatedAt).Minutes()
				responseTimes[msg.SenderID] = append(responseTimes[msg.SenderID], responseTime)
			}

			prevMsg = msg
		}
	}

	// Получаем информацию о чатах для каждого пользователя
	userChatsCount := make(map[int]int)

	// В ранее существующей реализации здесь был код для получения
	// количества чатов из OLAP базы. В новой реализации мы будем опираться
	// только на данные, передаваемые в метод.

	// Считаем количество чатов, где пользователь является покупателем или продавцом
	chatUserMap := make(map[int]map[int]bool) // map[userID]map[chatID]bool

	for _, msg := range messages {
		chatID := msg.ChatID
		senderID := msg.SenderID

		if _, exists := chatUserMap[senderID]; !exists {
			chatUserMap[senderID] = make(map[int]bool)
		}
		chatUserMap[senderID][chatID] = true
	}

	// Подсчитываем количество уникальных чатов для каждого пользователя
	for userID, chats := range chatUserMap {
		userChatsCount[userID] = len(chats)
	}

	// Обрабатываем каждого пользователя
	now := time.Now()

	for _, user := range users {
		// Рассчитываем days_active
		lastActivity, hasActivity := lastActivityMap[user.ID]
		var daysActive int
		if !hasActivity {
			// Если нет активности, считаем по текущей дате
			daysActive = int(math.Floor(now.Sub(user.CreatedAt).Hours() / 24))
		} else {
			// Если есть активность, считаем по последней активности
			daysActive = int(math.Floor(lastActivity.Sub(user.CreatedAt).Hours() / 24))
		}

		// Получаем количество сообщений
		totalMessages := totalMessagesMap[user.ID]

		// Рассчитываем среднее время ответа
		var avgResponseTime float64
		if times, exist := responseTimes[user.ID]; exist && len(times) > 0 {
			sum := 0.0
			for _, t := range times {
				sum += t
			}
			avgResponseTime = sum / float64(len(times))
		}

		// Определяем уровень активности
		// "high" - более 10 сообщений в неделю, "medium" - 3-10, "low" - менее 3
		var activityLevel string
		messagesPerWeek := 0.0
		if daysActive > 0 {
			messagesPerWeek = float64(totalMessages) / (float64(daysActive) / 7.0)
		}

		if messagesPerWeek > 10 {
			activityLevel = "high"
		} else if messagesPerWeek >= 3 {
			activityLevel = "medium"
		} else {
			activityLevel = "low"
		}

		// Получаем количество чатов
		totalChats := userChatsCount[user.ID]

		// Создаем объект UserDimension и добавляем его в результирующий список
		userDimension := models.UserDimension{
			ID:                     user.ID,
			RegistrationDate:       user.CreatedAt,
			DaysActive:             daysActive,
			TotalChats:             totalChats,
			TotalMessages:          totalMessages,
			AvgResponseTimeMinutes: avgResponseTime,
			ActivityLevel:          activityLevel,
			LastUpdated:            time.Now(),
		}

		transformedUsers = append(transformedUsers, userDimension)
	}

	p.logger.Info("Обработано измерение пользователей. Трансформировано записей: %d", len(transformedUsers))
	return transformedUsers, nil
}
