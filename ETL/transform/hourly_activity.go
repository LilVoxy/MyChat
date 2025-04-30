package transform

import (
	"database/sql"
	"fmt"
	"sort"
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

	// Получаем соответствие дат и часов к time_id через специальную функцию
	timeIDMap, err := p.getTimeIDMapping()
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении маппинга time_id: %w", err)
	}

	// Получаем данные о сообщениях для анализа
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
				// Если не найден ID, создаем новую запись в time_dimension
				t, _ := time.Parse("2006-01-02", dateStr)
				t = t.Add(time.Duration(hour) * time.Hour) // Устанавливаем правильный час

				var err error
				dateID, err = p.ensureTimeDimensionRecord(t)
				if err != nil {
					p.logger.Error("Не удалось создать запись в time_dimension для %s %d: %v", dateStr, hour, err)
					continue // Пропускаем этот час
				}

				// Обновляем маппинг
				if _, ok := timeIDMap[dateStr]; !ok {
					timeIDMap[dateStr] = make(map[int]int)
				}
				timeIDMap[dateStr][hour] = dateID
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
	rows, err := p.oltpDB.Query(`
		SELECT id, chat_id, sender_id, created_at
		FROM messages
		WHERE created_at >= ?
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []messageInfo
	for rows.Next() {
		var m messageInfo
		if err := rows.Scan(&m.id, &m.chatID, &m.senderID, &m.createdAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}

	// Группируем сообщения по чатам (создаем карту индексов, а не указателей)
	chatMessageIndices := make(map[int][]int)
	for i := range messages {
		chatMessageIndices[messages[i].chatID] = append(chatMessageIndices[messages[i].chatID], i)
	}

	// Для каждого чата сортируем сообщения по времени и вычисляем isFirstInChat/responseTime
	for _, indices := range chatMessageIndices {
		// Сортируем индексы по времени сообщений
		sort.Slice(indices, func(i, j int) bool {
			return messages[indices[i]].createdAt.Before(messages[indices[j]].createdAt)
		})

		// Помечаем первое сообщение и вычисляем время ответа
		for i, idx := range indices {
			if i == 0 {
				messages[idx].isFirstInChat = true
				messages[idx].responseTime = 0
			} else {
				messages[idx].isFirstInChat = false
				prevIdx := indices[i-1]
				if messages[idx].senderID != messages[prevIdx].senderID {
					messages[idx].responseTime = messages[idx].createdAt.Sub(messages[prevIdx].createdAt).Minutes()
				} else {
					messages[idx].responseTime = 0
				}
			}
		}
	}
	return messages, nil
}

// getTimeIDMapping получает маппинг дат и часов к time_id из OLAP базы данных
func (p *HourlyActivityProcessor) getTimeIDMapping() (map[string]map[int]int, error) {
	timeIDMap := make(map[string]map[int]int)

	rows, err := p.olapDB.Query(`
		SELECT id, full_date, hour_of_day
		FROM chat_analytics.time_dimension
		WHERE full_date >= DATE_SUB(CURDATE(), INTERVAL 7 DAY)
	`)
	if err != nil {
		return nil, fmt.Errorf("ошибка при запросе time_dimension: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, hour int
		var dateStr string
		if err := rows.Scan(&id, &dateStr, &hour); err != nil {
			return nil, err
		}
		if _, ok := timeIDMap[dateStr]; !ok {
			timeIDMap[dateStr] = make(map[int]int)
		}
		timeIDMap[dateStr][hour] = id
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

// ensureTimeDimensionRecord создает запись в time_dimension для указанного времени и возвращает ID
func (p *HourlyActivityProcessor) ensureTimeDimensionRecord(t time.Time) (int, error) {
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
