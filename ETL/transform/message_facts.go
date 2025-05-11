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

	// Проверяем, для всех ли сообщений есть соответствующие чаты
	var missingChats []int
	for chatID := range chatMessages {
		if _, exists := chatMap[chatID]; !exists {
			missingChats = append(missingChats, chatID)
		}
	}

	// Если есть сообщения без соответствующих чатов, запрашиваем недостающие чаты из базы
	if len(missingChats) > 0 {
		p.logger.Debug("Найдено %d чатов, отсутствующих в извлеченных данных. Запрашиваем дополнительно.", len(missingChats))

		placeholders := make([]string, len(missingChats))
		args := make([]interface{}, len(missingChats))
		for i, chatID := range missingChats {
			placeholders[i] = "?"
			args[i] = chatID
		}

		query := fmt.Sprintf(`
			SELECT id, buyer_id, seller_id, created_at 
			FROM chats 
			WHERE id IN (%s)`, strings.Join(placeholders, ","))

		rows, err := p.oltpDB.Query(query, args...)
		if err != nil {
			p.logger.Error("Ошибка при запросе дополнительных чатов: %v", err)
			// Продолжаем выполнение, но это может привести к неполной обработке
		} else {
			defer rows.Close()
			for rows.Next() {
				var chat models.ChatOLTP
				err := rows.Scan(&chat.ID, &chat.BuyerID, &chat.SellerID, &chat.CreatedAt)
				if err != nil {
					p.logger.Error("Ошибка при сканировании данных чата: %v", err)
					continue
				}
				chatMap[chat.ID] = chat
			}

			p.logger.Debug("Дополнительно получено %d чатов из базы данных", len(chatMap)-len(chats))
		}
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
			p.logger.Debug("Чат с ID %d не найден даже после дополнительного запроса. Пропускаем обработку сообщений этого чата.", chatID)
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
			timeID := p.getTimeID(timeIDMap, dateKey, 0)

			// Если не удалось найти time_id, создаем новую запись в time_dimension
			if timeID == 0 {
				var err error
				timeID, err = p.ensureTimeDimensionRecord(msg.CreatedAt)
				if err != nil {
					p.logger.Error("Не удалось создать запись в time_dimension для сообщения %d: %v", msg.ID, err)
					continue // Пропускаем это сообщение
				}

				// Обновляем маппинг
				timeIDMap[dateKey] = timeID
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

// getTimeIDMapping получает маппинг дат к time_id из OLAP базы данных
func (p *MessageFactsProcessor) getTimeIDMapping() (map[string]int, error) {
	timeIDMap := make(map[string]int)

	rows, err := p.olapDB.Query(`
		SELECT id, full_date
		FROM chat_analytics.time_dimension
		WHERE full_date >= DATE_SUB(CURDATE(), INTERVAL 30 DAY)
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
func (p *MessageFactsProcessor) getTimeID(timeIDMap map[string]int, date string, _ int) int {
	if timeID, exists := timeIDMap[date]; exists {
		return timeID
	}
	return 0 // Возвращаем 0, если не найдено
}

// ensureTimeDimensionRecord создает запись в time_dimension для указанного времени и возвращает ID
func (p *MessageFactsProcessor) ensureTimeDimensionRecord(t time.Time) (int, error) {
	// Проверяем, существует ли уже запись (хотя мы должны были проверить это через getTimeID)
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
