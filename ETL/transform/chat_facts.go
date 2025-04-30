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

	// Группируем сообщения по чатам для анализа
	chatMessages := make(map[int][]models.MessageOLTP)
	for _, msg := range messages {
		chatMessages[msg.ChatID] = append(chatMessages[msg.ChatID], msg)
	}

	// Для каждого чата сортируем сообщения по времени
	for chatID, msgs := range chatMessages {
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
				// Если не найден ID, создаем новую запись в time_dimension
				var err error
				startTimeID, err = p.ensureTimeDimensionRecord(chat.CreatedAt)
				if err != nil {
					p.logger.Error("Не удалось создать запись в time_dimension для чата %d: %v", chat.ID, err)
					continue // Пропускаем этот чат
				}

				// Обновляем маппинг
				if _, ok := timeIDMap[dateKey]; !ok {
					timeIDMap[dateKey] = make(map[int]int)
				}
				timeIDMap[dateKey][hourKey] = startTimeID
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
				// Если не найден ID, создаем новую запись в time_dimension
				var err error
				startTimeID, err = p.ensureTimeDimensionRecord(firstMsg.CreatedAt)
				if err != nil {
					p.logger.Error("Не удалось создать запись в time_dimension для старта чата %d: %v", chat.ID, err)
					continue // Пропускаем этот чат
				}

				// Обновляем маппинг
				if _, ok := timeIDMap[firstDateKey]; !ok {
					timeIDMap[firstDateKey] = make(map[int]int)
				}
				timeIDMap[firstDateKey][firstHourKey] = startTimeID
			}

			// Получаем время последнего сообщения
			lastDateKey := lastMsg.CreatedAt.Format("2006-01-02")
			lastHourKey := lastMsg.CreatedAt.Hour()
			endTimeID = p.getTimeID(timeIDMap, lastDateKey, lastHourKey)

			if endTimeID == 0 {
				// Если не найден ID, создаем новую запись в time_dimension
				var err error
				endTimeID, err = p.ensureTimeDimensionRecord(lastMsg.CreatedAt)
				if err != nil {
					p.logger.Error("Не удалось создать запись в time_dimension для окончания чата %d: %v", chat.ID, err)
					continue // Пропускаем этот чат
				}

				// Обновляем маппинг
				if _, ok := timeIDMap[lastDateKey]; !ok {
					timeIDMap[lastDateKey] = make(map[int]int)
				}
				timeIDMap[lastDateKey][lastHourKey] = endTimeID
			}

			// Рассчитываем длительность чата в часах
			if len(msgs) <= 1 {
				chatDurationHours = 0
			} else {
				chatDurationHours = lastMsg.CreatedAt.Sub(firstMsg.CreatedAt).Hours()
			}

			// Рассчитываем метрики сообщений
			totalMessages = len(msgs)
			for _, msg := range msgs {
				// Считаем сообщения от покупателя и продавца
				if msg.SenderID == chat.BuyerID {
					buyerMessages++
				} else if msg.SenderID == chat.SellerID {
					sellerMessages++
				}

				// Считаем общую длину сообщений
				totalLength += len(strings.TrimSpace(msg.Message))
			}

			// Рассчитываем время ответа для каждого сообщения
			for i := 1; i < len(msgs); i++ {
				// Если отправитель отличается от предыдущего сообщения, это ответ
				if msgs[i].SenderID != msgs[i-1].SenderID {
					// Вычисляем время ответа в минутах
					responseTime := msgs[i].CreatedAt.Sub(msgs[i-1].CreatedAt).Minutes()
					responseTimes = append(responseTimes, responseTime)
				}
			}
		}

		// Рассчитываем средние значения
		var avgMessageLength float64
		if totalMessages > 0 {
			avgMessageLength = float64(totalLength) / float64(totalMessages)
		}

		var avgResponseTime float64
		if len(responseTimes) > 0 {
			// Вычисляем среднее время ответа
			var sum float64
			for _, rt := range responseTimes {
				sum += rt
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

// ensureTimeDimensionRecord создает запись в time_dimension для указанного времени и возвращает ID
func (p *ChatFactsProcessor) ensureTimeDimensionRecord(t time.Time) (int, error) {
	// Проверяем, существует ли уже запись (хотя мы должны были проверить это через getTimeID)
	var id int
	err := p.olapDB.QueryRow(`
		SELECT id FROM chat_analytics.time_dimension 
		WHERE full_date = ? AND hour_of_day = ?
	`, t.Format("2006-01-02"), t.Hour()).Scan(&id)

	if err == nil {
		// Запись уже существует
		return id, nil
	} else if err != sql.ErrNoRows {
		// Произошла ошибка, отличная от "записи не найдены"
		return 0, err
	}

	// Создаем новую запись
	// Определяем компоненты даты
	year := t.Year()
	month := int(t.Month())
	monthNames := []string{"January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December"}
	monthName := monthNames[month-1]

	// Определяем квартал
	quarter := (month-1)/3 + 1

	// Номер недели в году (приблизительно)
	yearDay := t.YearDay()
	weekOfYear := (yearDay-1)/7 + 1

	dayOfMonth := t.Day()
	dayOfWeek := int(t.Weekday()) + 1 // 1=Sunday, 7=Saturday
	dayNames := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	dayName := dayNames[dayOfWeek-1]

	// Выходной день (суббота или воскресенье)
	isWeekend := dayOfWeek == 1 || dayOfWeek == 7

	hourOfDay := t.Hour()

	// Вставляем запись
	result, err := p.olapDB.Exec(`
		INSERT INTO chat_analytics.time_dimension 
		(full_date, year, quarter, month, month_name, week_of_year, 
		day_of_month, day_of_week, day_name, is_weekend, hour_of_day) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		t.Format("2006-01-02"), // full_date
		year,
		quarter,
		month,
		monthName,
		weekOfYear,
		dayOfMonth,
		dayOfWeek,
		dayName,
		isWeekend,
		hourOfDay,
	)

	if err != nil {
		return 0, fmt.Errorf("ошибка при создании записи в time_dimension: %w", err)
	}

	// Получаем ID вставленной записи
	lastID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ошибка при получении ID новой записи time_dimension: %w", err)
	}

	p.logger.Debug("Создана новая запись в time_dimension для даты %s, часа %d, ID: %d",
		t.Format("2006-01-02"), hourOfDay, lastID)
	return int(lastID), nil
}
