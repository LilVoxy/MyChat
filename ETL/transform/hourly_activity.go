package transform

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/models"
	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// HourlyActivityProcessor отвечает за обработку почасовой активности
type HourlyActivityProcessor struct {
	oltpDB *sql.DB
	olapDB *sql.DB
	logger *utils.ETLLogger
}

// NewHourlyActivityProcessor создает новый экземпляр HourlyActivityProcessor
func NewHourlyActivityProcessor(oltpDB, olapDB *sql.DB, logger *utils.ETLLogger) *HourlyActivityProcessor {
	return &HourlyActivityProcessor{
		oltpDB: oltpDB,
		olapDB: olapDB,
		logger: logger,
	}
}

// ProcessHourlyActivity обрабатывает данные почасовой активности и возвращает трансформированные факты
func (p *HourlyActivityProcessor) ProcessHourlyActivity() ([]models.HourlyActivityFact, error) {
	p.logger.Debug("Обработка почасовой активности...")

	// Получаем последние 7 дней для обновления агрегатов
	// В реальной системе можно было бы обрабатывать только те часы,
	// для которых есть новые данные
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)

	// Результирующий список фактов почасовой активности
	hourlyFacts := make([]models.HourlyActivityFact, 0)

	// Получаем данные о сообщениях из OLAP для агрегации
	// В реальной имплементации мы бы сделали запрос к OLAP базе
	// Для прототипа сгенерируем синтетические данные

	// Получаем соответствие дат и часов к time_id через специальную функцию
	timeIDMap, err := p.getTimeIDMapping()
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении маппинга time_id: %w", err)
	}

	// Получаем данные о сообщениях для анализа
	// Здесь мы генерируем синтетические данные
	messageData, err := p.getMessageData(sevenDaysAgo)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении данных о сообщениях: %w", err)
	}

	// Группируем данные по датам и часам
	hourlyData := make(map[string]map[int]struct {
		messages      int
		newChats      int
		activeUsers   map[int]bool
		responseTimes []float64
	})

	// Инициализируем структуру данных
	now := time.Now()
	for i := 0; i < 7; i++ {
		date := now.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		hourlyData[dateStr] = make(map[int]struct {
			messages      int
			newChats      int
			activeUsers   map[int]bool
			responseTimes []float64
		})

		for h := 0; h < 24; h++ {
			hourlyData[dateStr][h] = struct {
				messages      int
				newChats      int
				activeUsers   map[int]bool
				responseTimes []float64
			}{
				messages:      0,
				newChats:      0,
				activeUsers:   make(map[int]bool),
				responseTimes: make([]float64, 0),
			}
		}
	}

	// Распределяем сообщения по часам
	for _, msg := range messageData {
		dateStr := msg.createdAt.Format("2006-01-02")
		hour := msg.createdAt.Hour()

		// Пропускаем сообщения вне интересующего нас периода
		if _, exists := hourlyData[dateStr]; !exists {
			continue
		}
		if _, exists := hourlyData[dateStr][hour]; !exists {
			continue
		}

		// Увеличиваем счетчик сообщений
		data := hourlyData[dateStr][hour]
		data.messages++

		// Отмечаем активного пользователя
		data.activeUsers[msg.senderID] = true

		// Если это первое сообщение в чате, увеличиваем счетчик новых чатов
		if msg.isFirstInChat {
			data.newChats++
		}

		// Добавляем время ответа, если это ответное сообщение
		if msg.responseTime > 0 {
			data.responseTimes = append(data.responseTimes, msg.responseTime)
		}

		hourlyData[dateStr][hour] = data
	}

	// Формируем факты почасовой активности
	for dateStr, hourMap := range hourlyData {
		for hour, data := range hourMap {
			// Пропускаем часы без активности
			if data.messages == 0 {
				continue
			}

			// Рассчитываем среднее время ответа
			var avgResponseTime float64
			if len(data.responseTimes) > 0 {
				sum := 0.0
				for _, rt := range data.responseTimes {
					sum += rt
				}
				avgResponseTime = sum / float64(len(data.responseTimes))
			}

			// Получаем time_id из маппинга
			dateID := p.getTimeID(timeIDMap, dateStr, hour)
			if dateID == 0 {
				// Если не найден ID, используем временный ID
				t, _ := time.Parse("2006-01-02", dateStr)
				dateID = -1 * (int(t.Unix()) % 10000) // Отрицательный ID для индикации временного значения
				p.logger.Debug("Использован временный date_id для %s %d: %d", dateStr, hour, dateID)
			}

			// Создаем факт почасовой активности
			hourlyFact := models.HourlyActivityFact{
				DateID:                 dateID,
				HourOfDay:              hour,
				TotalMessages:          data.messages,
				TotalNewChats:          data.newChats,
				ActiveUsers:            len(data.activeUsers),
				AvgResponseTimeMinutes: avgResponseTime,
			}

			// Добавляем факт в результат
			hourlyFacts = append(hourlyFacts, hourlyFact)
		}
	}

	p.logger.Info("Обработка почасовой активности завершена. Сформировано фактов: %d", len(hourlyFacts))
	return hourlyFacts, nil
}

// Вспомогательная структура для хранения данных о сообщениях
type messageInfo struct {
	id            int
	chatID        int
	senderID      int
	createdAt     time.Time
	isFirstInChat bool
	responseTime  float64
}

// getMessageData получает данные о сообщениях для анализа
func (p *HourlyActivityProcessor) getMessageData(since time.Time) ([]messageInfo, error) {
	// В реальной имплементации здесь был бы запрос к OLAP базе
	// Для прототипа создаем синтетические данные

	// Генерируем случайные данные о сообщениях за последние 7 дней
	messages := make([]messageInfo, 0)
	now := time.Now()

	// Создаем 10 случайных чатов
	chatIDs := []int{1001, 1002, 1003, 1004, 1005, 1006, 1007, 1008, 1009, 1010}

	// Создаем 5 пользователей
	userIDs := []int{101, 102, 103, 104, 105}

	// Создаем данные за последние 7 дней с разной интенсивностью по часам
	for day := 0; day < 7; day++ {
		date := now.AddDate(0, 0, -day)

		// Распределение активности по часам (больше днем, меньше ночью)
		hourlyDistribution := []int{1, 0, 0, 0, 1, 2, 5, 10, 15, 20, 15, 10, 20, 25, 20, 15, 10, 15, 10, 5, 3, 2, 1, 0}

		for hour, count := range hourlyDistribution {
			chatActive := make(map[int]bool)
			lastSender := make(map[int]int) // chatID -> lastSenderID

			for i := 0; i < count; i++ {
				// Выбираем чат
				chatID := chatIDs[i%len(chatIDs)]

				// Выбираем отправителя
				senderID := userIDs[i%len(userIDs)]

				// Создаем время сообщения
				msgTime := time.Date(date.Year(), date.Month(), date.Day(), hour, i*2, 0, 0, time.UTC)

				// Определяем, является ли сообщение первым в чате
				isFirst := !chatActive[chatID]
				if isFirst {
					chatActive[chatID] = true
					lastSender[chatID] = senderID
				}

				// Рассчитываем время ответа
				var responseTime float64
				if !isFirst && lastSender[chatID] != senderID {
					// Если это ответ (другой отправитель), устанавливаем время ответа от 1 до 10 минут
					responseTime = 1 + float64(i%10)
				}

				// Запоминаем последнего отправителя
				lastSender[chatID] = senderID

				// Создаем запись о сообщении
				msg := messageInfo{
					id:            10000 + day*100 + hour*10 + i,
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

	return messages, nil
}

// getTimeIDMapping получает маппинг дат и часов к time_id из OLAP базы данных
func (p *HourlyActivityProcessor) getTimeIDMapping() (map[string]map[int]int, error) {
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
func (p *HourlyActivityProcessor) getTimeID(timeIDMap map[string]map[int]int, date string, hour int) int {
	if hourMap, dateExists := timeIDMap[date]; dateExists {
		if timeID, hourExists := hourMap[hour]; hourExists {
			return timeID
		}
	}
	return 0 // Возвращаем 0, если не найдено
}
