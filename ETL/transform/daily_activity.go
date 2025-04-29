package transform

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/models"
	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// DailyActivityProcessor отвечает за обработку ежедневной активности
type DailyActivityProcessor struct {
	oltpDB *sql.DB
	olapDB *sql.DB
	logger *utils.ETLLogger
}

// NewDailyActivityProcessor создает новый экземпляр DailyActivityProcessor
func NewDailyActivityProcessor(oltpDB, olapDB *sql.DB, logger *utils.ETLLogger) *DailyActivityProcessor {
	return &DailyActivityProcessor{
		oltpDB: oltpDB,
		olapDB: olapDB,
		logger: logger,
	}
}

// ProcessDailyActivity обрабатывает данные ежедневной активности и возвращает трансформированные факты
func (p *DailyActivityProcessor) ProcessDailyActivity() ([]models.DailyActivityFact, error) {
	p.logger.Debug("Обработка ежедневной активности...")

	// Получаем последние 30 дней для обновления агрегатов
	// В реальной системе можно было бы обрабатывать только те дни,
	// для которых есть новые данные
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)

	// Результирующий список фактов ежедневной активности
	dailyFacts := make([]models.DailyActivityFact, 0)

	// Получаем данные о сообщениях и пользователях для анализа
	// В реальной имплементации мы бы делали запросы к OLAP базе
	// Для прототипа сгенерируем синтетические данные

	// Получаем соответствие дат к time_id через специальную функцию
	timeIDMap, err := p.getTimeIDMapping()
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении маппинга time_id: %w", err)
	}

	// Получаем данные о сообщениях и пользователях
	messageData, userRegData, err := p.getActivityData(thirtyDaysAgo)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении данных активности: %w", err)
	}

	// Группируем данные по датам
	dailyData := make(map[string]struct {
		messages           map[int][]messageInfo // chatID -> []messageInfo
		newChats           int
		activeUsers        map[int]bool // userID -> bool
		newUsers           int
		messageCountByHour map[int]int // hour -> count
	})

	// Инициализируем структуру данных
	now := time.Now()
	for i := 0; i < 30; i++ {
		date := now.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		dailyData[dateStr] = struct {
			messages           map[int][]messageInfo
			newChats           int
			activeUsers        map[int]bool
			newUsers           int
			messageCountByHour map[int]int
		}{
			messages:           make(map[int][]messageInfo),
			newChats:           0,
			activeUsers:        make(map[int]bool),
			newUsers:           0,
			messageCountByHour: make(map[int]int),
		}
	}

	// Распределяем данные о новых пользователях
	for _, regDate := range userRegData {
		dateStr := regDate.Format("2006-01-02")
		if data, exists := dailyData[dateStr]; exists {
			data.newUsers++
			dailyData[dateStr] = data
		}
	}

	// Распределяем сообщения по датам
	for _, msg := range messageData {
		dateStr := msg.createdAt.Format("2006-01-02")
		if data, exists := dailyData[dateStr]; exists {
			// Добавляем сообщение в соответствующий чат
			if _, ok := data.messages[msg.chatID]; !ok {
				data.messages[msg.chatID] = make([]messageInfo, 0)
			}
			data.messages[msg.chatID] = append(data.messages[msg.chatID], msg)

			// Отмечаем активного пользователя
			data.activeUsers[msg.senderID] = true

			// Если это первое сообщение в чате, увеличиваем счетчик новых чатов
			if msg.isFirstInChat {
				data.newChats++
			}

			// Увеличиваем счетчик сообщений для часа
			hour := msg.createdAt.Hour()
			data.messageCountByHour[hour]++

			dailyData[dateStr] = data
		}
	}

	// Формируем факты ежедневной активности
	for dateStr, data := range dailyData {
		// Пропускаем дни без активности
		totalMessages := 0
		for _, chatMsgs := range data.messages {
			totalMessages += len(chatMsgs)
		}

		if totalMessages == 0 && data.newUsers == 0 {
			continue
		}

		// Рассчитываем среднее количество сообщений в чате
		var avgMessagesPerChat float64
		if len(data.messages) > 0 {
			avgMessagesPerChat = float64(totalMessages) / float64(len(data.messages))
		}

		// Рассчитываем среднее время ответа
		var totalResponseTime float64
		var responseCount int
		for _, chatMsgs := range data.messages {
			for _, msg := range chatMsgs {
				if msg.responseTime > 0 {
					totalResponseTime += msg.responseTime
					responseCount++
				}
			}
		}

		var avgResponseTime float64
		if responseCount > 0 {
			avgResponseTime = totalResponseTime / float64(responseCount)
		}

		// Определяем пиковый час
		peakHour := 0
		peakHourMessages := 0
		for hour, count := range data.messageCountByHour {
			if count > peakHourMessages {
				peakHour = hour
				peakHourMessages = count
			}
		}

		// Получаем time_id из маппинга
		dateID := p.getTimeID(timeIDMap, dateStr)
		if dateID == 0 {
			// Если не найден ID, используем временный ID
			t, _ := time.Parse("2006-01-02", dateStr)
			dateID = -1 * (int(t.Unix()) % 10000) // Отрицательный ID для индикации временного значения
			p.logger.Debug("Использован временный date_id для %s: %d", dateStr, dateID)
		}

		// Создаем факт ежедневной активности
		dailyFact := models.DailyActivityFact{
			DateID:                 dateID,
			TotalMessages:          totalMessages,
			TotalNewChats:          data.newChats,
			ActiveUsers:            len(data.activeUsers),
			NewUsers:               data.newUsers,
			AvgMessagesPerChat:     avgMessagesPerChat,
			AvgResponseTimeMinutes: avgResponseTime,
			PeakHour:               peakHour,
			PeakHourMessages:       peakHourMessages,
		}

		// Добавляем факт в результат
		dailyFacts = append(dailyFacts, dailyFact)
	}

	p.logger.Info("Обработка ежедневной активности завершена. Сформировано фактов: %d", len(dailyFacts))
	return dailyFacts, nil
}

// getActivityData получает данные о сообщениях и регистрациях пользователей для анализа
func (p *DailyActivityProcessor) getActivityData(since time.Time) ([]messageInfo, []time.Time, error) {
	// В реальной имплементации здесь были бы запросы к OLAP базе
	// Для прототипа создаем синтетические данные

	// Генерируем случайные данные о сообщениях за последние 30 дней
	messages := make([]messageInfo, 0)
	registrationDates := make([]time.Time, 0)
	now := time.Now()

	// Создаем 15 случайных чатов
	chatIDs := []int{1001, 1002, 1003, 1004, 1005, 1006, 1007, 1008, 1009, 1010, 1011, 1012, 1013, 1014, 1015}

	// Создаем 20 пользователей
	userIDs := []int{101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120}

	// Создаем даты регистрации для 20 пользователей, распределенные по 30 дням
	for i := 0; i < 20; i++ {
		day := i % 30
		regDate := now.AddDate(0, 0, -day)
		registrationDates = append(registrationDates, regDate)
	}

	// Создаем данные за последние 30 дней с разной интенсивностью по дням
	// Интенсивность выше ближе к текущей дате
	for day := 0; day < 30; day++ {
		date := now.AddDate(0, 0, -day)

		// Количество сообщений в день уменьшается с давностью
		dailyMessageCount := 100 - day*3
		if dailyMessageCount < 10 {
			dailyMessageCount = 10
		}

		// Распределение активности по часам (больше днем, меньше ночью)
		hourlyDistribution := []int{1, 0, 0, 0, 1, 2, 5, 10, 15, 20, 15, 10, 20, 25, 20, 15, 10, 15, 10, 5, 3, 2, 1, 0}

		chatActive := make(map[int]bool)
		lastSender := make(map[int]int) // chatID -> lastSenderID

		// Распределяем сообщения по часам и чатам
		hourlyMessages := make(map[int]int) // час -> количество сообщений
		for i := 0; i < dailyMessageCount; i++ {
			// Выбираем случайный час на основе распределения
			hourIdx := i % len(hourlyDistribution)
			hour := hourIdx
			messageCount := hourlyDistribution[hourIdx]

			hourlyMessages[hour] += messageCount

			for j := 0; j < messageCount; j++ {
				// Выбираем чат (используем индекс для распределения)
				chatIdx := (i + j) % len(chatIDs)
				chatID := chatIDs[chatIdx]

				// Выбираем отправителя (используем индекс для распределения)
				senderIdx := (i + j*2) % len(userIDs)
				senderID := userIDs[senderIdx]

				// Создаем время сообщения, распределяя минуты
				minute := (j * 60 / messageCount) % 60
				msgTime := time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, time.UTC)

				// Определяем, является ли сообщение первым в чате для этого дня
				isFirst := !chatActive[chatID]
				if isFirst {
					chatActive[chatID] = true
					lastSender[chatID] = senderID
				}

				// Рассчитываем время ответа
				var responseTime float64
				if !isFirst && lastSender[chatID] != senderID {
					// Если это ответ (другой отправитель), устанавливаем время ответа от 1 до 15 минут
					responseTime = 1 + float64((i+j)%15)
				}

				// Запоминаем последнего отправителя
				lastSender[chatID] = senderID

				// Создаем запись о сообщении
				msg := messageInfo{
					id:            10000 + day*1000 + hour*10 + j,
					chatID:        chatID,
					senderID:      senderID,
					createdAt:     msgTime,
					isFirstInChat: isFirst,
					responseTime:  responseTime,
				}

				messages = append(messages, msg)
			}
		}
	}

	return messages, registrationDates, nil
}

// getTimeIDMapping получает маппинг дат к time_id из OLAP базы данных
func (p *DailyActivityProcessor) getTimeIDMapping() (map[string]int, error) {
	timeIDMap := make(map[string]int)

	// В реальной имплементации здесь был бы запрос к OLAP базе для получения маппинга
	// Для прототипа создаем тестовые данные

	// Создаем данные для последних 30 дней (для примера)
	now := time.Now()
	for i := 0; i < 30; i++ {
		date := now.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")

		// Генерируем простой ID на основе даты
		// В реальной системе ID были бы получены из базы данных
		timeIDMap[dateStr] = 1000 + i
	}

	return timeIDMap, nil
}

// getTimeID возвращает time_id для указанной даты из маппинга
func (p *DailyActivityProcessor) getTimeID(timeIDMap map[string]int, date string) int {
	if timeID, exists := timeIDMap[date]; exists {
		return timeID
	}
	return 0 // Возвращаем 0, если не найдено
}
