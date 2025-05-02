package transform

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/models"
	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// Вспомогательная структура для хранения данных о сообщениях
type messageInfo struct {
	id            int
	chatID        int
	senderID      int
	createdAt     time.Time
	isFirstInChat bool
	responseTime  float64
}

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

	// Результирующий список фактов ежедневной активности
	dailyFacts := make([]models.DailyActivityFact, 0)

	// Получаем соответствие дат к time_id через специальную функцию
	timeIDMap, err := p.getTimeIDMapping()
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении маппинга time_id: %w", err)
	}

	// Получаем данные о последнем запуске ETL для определения начальной точки
	lastProcessedMessageID, err := p.getLastProcessedMessageID()
	if err != nil {
		p.logger.Error("Ошибка при получении последнего обработанного сообщения: %v. Будут обработаны все данные.", err)
		lastProcessedMessageID = 0
	}

	p.logger.Info("Начало обработки данных начиная с ID сообщения: %d", lastProcessedMessageID)

	// Получаем данные о сообщениях и пользователях, начиная с последнего обработанного ID
	messageData, userRegData, err := p.getActivityDataFromLastID(lastProcessedMessageID)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении данных активности: %w", err)
	}

	p.logger.Info("Получено сообщений: %d, данных о регистрациях: %d", len(messageData), len(userRegData))

	// Определяем полный диапазон дат из полученных данных
	dateRange := make(map[string]bool)
	for _, msg := range messageData {
		dateStr := msg.createdAt.Format("2006-01-02")
		dateRange[dateStr] = true
	}
	for _, regDate := range userRegData {
		dateStr := regDate.Format("2006-01-02")
		dateRange[dateStr] = true
	}

	// Если данных нет, возвращаем пустой результат
	if len(dateRange) == 0 {
		p.logger.Info("Нет новых данных для обработки")
		return dailyFacts, nil
	}

	// Группируем данные по датам
	dailyData := make(map[string]struct {
		messages           map[int][]messageInfo // chatID -> []messageInfo
		newChats           int
		activeUsers        map[int]bool // userID -> bool
		newUsers           int
		messageCountByHour map[int]int // hour -> count
	})

	// Инициализируем структуру данных для всех дат в диапазоне
	for dateStr := range dateRange {
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

	// Карта для отслеживания уже обработанных date_id, чтобы избежать дубликатов
	processedDateIDs := make(map[int]bool)

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
			// Если не найден ID, создаем новую запись в time_dimension
			t, _ := time.Parse("2006-01-02", dateStr)
			var err error
			dateID, err = p.ensureTimeDimensionRecord(t)
			if err != nil {
				p.logger.Error("Не удалось создать запись в time_dimension для даты %s: %v", dateStr, err)
				continue // Пропускаем эту дату
			}

			// Обновляем маппинг
			timeIDMap[dateStr] = dateID
		}

		// Проверяем, не был ли этот date_id уже обработан
		if processedDateIDs[dateID] {
			p.logger.Debug("Пропуск даты %s (ID: %d), так как она уже обработана", dateStr, dateID)
			continue
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

		// Отмечаем этот date_id как обработанный
		processedDateIDs[dateID] = true

		// Добавляем факт в результат
		dailyFacts = append(dailyFacts, dailyFact)
	}

	p.logger.Info("Обработка ежедневной активности завершена. Сформировано фактов: %d", len(dailyFacts))
	return dailyFacts, nil
}

// getLastProcessedMessageID получает ID последнего обработанного сообщения из журнала ETL
func (p *DailyActivityProcessor) getLastProcessedMessageID() (int, error) {
	var lastID int
	err := p.olapDB.QueryRow(`
		SELECT last_processed_message_id 
		FROM etl_runs 
		WHERE status = 'success' 
		ORDER BY end_time DESC 
		LIMIT 1
	`).Scan(&lastID)

	if err != nil {
		return 0, err
	}
	return lastID, nil
}

// getActivityDataFromLastID получает данные о сообщениях и регистрациях пользователей для анализа
// начиная с последнего обработанного ID сообщения
func (p *DailyActivityProcessor) getActivityDataFromLastID(lastProcessedMessageID int) ([]messageInfo, []time.Time, error) {
	// Получаем сообщения из OLTP начиная с указанного ID
	rows, err := p.oltpDB.Query(`
		SELECT id, chat_id, sender_id, created_at
		FROM messages
		WHERE id > ?
		ORDER BY id
	`, lastProcessedMessageID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var messages []messageInfo
	for rows.Next() {
		var m messageInfo
		if err := rows.Scan(&m.id, &m.chatID, &m.senderID, &m.createdAt); err != nil {
			return nil, nil, err
		}
		messages = append(messages, m)
	}

	// Если нет новых сообщений, получаем хотя бы минимальную дату для регистраций пользователей
	var minProcessedDate time.Time
	if len(messages) > 0 {
		// Находим самую раннюю дату среди новых сообщений
		minProcessedDate = messages[0].createdAt
		for _, msg := range messages {
			if msg.createdAt.Before(minProcessedDate) {
				minProcessedDate = msg.createdAt
			}
		}
	} else {
		// Если сообщений нет, используем дату из последнего успешного запуска
		err := p.olapDB.QueryRow(`
			SELECT end_time 
			FROM etl_runs 
			WHERE status = 'success' 
			ORDER BY end_time DESC 
			LIMIT 1
		`).Scan(&minProcessedDate)

		if err != nil {
			// Если и это не удалось, берем дату на месяц назад
			minProcessedDate = time.Now().AddDate(0, -1, 0)
		}
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

	// Получаем даты регистрации пользователей, начиная с минимальной даты обработки
	userRows, err := p.oltpDB.Query(`
		SELECT created_at FROM users WHERE created_at >= ?
	`, minProcessedDate)
	if err != nil {
		return nil, nil, err
	}
	defer userRows.Close()

	var registrationDates []time.Time
	for userRows.Next() {
		var reg time.Time
		if err := userRows.Scan(&reg); err != nil {
			return nil, nil, err
		}
		registrationDates = append(registrationDates, reg)
	}

	return messages, registrationDates, nil
}

// getActivityData получает данные о сообщениях и регистрациях пользователей для анализа
// УСТАРЕВШИЙ метод, использовался для получения данных за фиксированный период
func (p *DailyActivityProcessor) getActivityData(since time.Time) ([]messageInfo, []time.Time, error) {
	// Получаем сообщения из OLTP
	rows, err := p.oltpDB.Query(`
		SELECT id, chat_id, sender_id, created_at
		FROM messages
		WHERE created_at >= ?
	`, since)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var messages []messageInfo
	for rows.Next() {
		var m messageInfo
		if err := rows.Scan(&m.id, &m.chatID, &m.senderID, &m.createdAt); err != nil {
			return nil, nil, err
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

	// Получаем даты регистрации пользователей
	userRows, err := p.oltpDB.Query(`
		SELECT created_at FROM users WHERE created_at >= ?
	`, since)
	if err != nil {
		return nil, nil, err
	}
	defer userRows.Close()

	var registrationDates []time.Time
	for userRows.Next() {
		var reg time.Time
		if err := userRows.Scan(&reg); err != nil {
			return nil, nil, err
		}
		registrationDates = append(registrationDates, reg)
	}

	return messages, registrationDates, nil
}

// getTimeIDMapping получает маппинг дат к time_id из OLAP базы данных
func (p *DailyActivityProcessor) getTimeIDMapping() (map[string]int, error) {
	timeIDMap := make(map[string]int)

	// Получаем все записи из time_dimension, а не только за последние 30 дней
	rows, err := p.olapDB.Query(`
		SELECT id, full_date
		FROM chat_analytics.time_dimension
	`)
	if err != nil {
		return nil, fmt.Errorf("ошибка при запросе time_dimension: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var dateStr string
		if err := rows.Scan(&id, &dateStr); err != nil {
			return nil, err
		}
		timeIDMap[dateStr] = id
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

// ensureTimeDimensionRecord создает запись в time_dimension для указанной даты и возвращает ID
func (p *DailyActivityProcessor) ensureTimeDimensionRecord(t time.Time) (int, error) {
	// Запрос для поиска существующей записи для данной даты
	var id int
	err := p.olapDB.QueryRow(`
		SELECT id FROM chat_analytics.time_dimension 
		WHERE full_date = ?
	`, t.Format("2006-01-02")).Scan(&id)

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

	// Вставляем запись
	result, err := p.olapDB.Exec(`
		INSERT INTO chat_analytics.time_dimension 
		(full_date, year, quarter, month, month_name, week_of_year, 
		day_of_month, day_of_week, day_name, is_weekend) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
	)

	if err != nil {
		return 0, fmt.Errorf("ошибка при создании записи в time_dimension: %w", err)
	}

	// Получаем ID вставленной записи
	lastID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ошибка при получении ID новой записи time_dimension: %w", err)
	}

	p.logger.Debug("Создана новая запись в time_dimension для даты %s, ID: %d",
		t.Format("2006-01-02"), lastID)
	return int(lastID), nil
}
