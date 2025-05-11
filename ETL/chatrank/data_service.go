package chatrank

import (
	"database/sql"
	"fmt"

	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// MySQLDataService реализация сервиса для работы с данными из MySQL
type MySQLDataService struct {
	oltpDB *sql.DB
	olapDB *sql.DB
	logger *utils.ETLLogger
}

// NewMySQLDataService создает новый экземпляр MySQLDataService
func NewMySQLDataService(oltpDB, olapDB *sql.DB, logger *utils.ETLLogger) *MySQLDataService {
	return &MySQLDataService{
		oltpDB: oltpDB,
		olapDB: olapDB,
		logger: logger,
	}
}

// GetMessagesForChatRank получает данные о сообщениях для расчета ChatRank
func (s *MySQLDataService) GetMessagesForChatRank() (map[int]map[int][]MessageInfo, error) {
	// Результирующая структура данных
	messagesMap := make(map[int]map[int][]MessageInfo)

	s.logger.Info("Запрос данных для расчета ChatRank")

	// Запрос для получения данных из OLAP-куба
	// Используем таблицу message_facts, которая уже содержит нужные метрики
	rows, err := s.olapDB.Query(`
		SELECT 
			mf.sender_id,
			mf.recipient_id,
			mf.message_length,
			mf.response_time_minutes,
			CASE WHEN EXISTS (
				SELECT 1 FROM chat_analytics.message_facts mf2 
				WHERE mf2.recipient_id = mf.sender_id AND mf2.sender_id = mf.recipient_id
				AND mf2.chat_id = mf.chat_id
			) THEN 1 ELSE 0 END AS has_response,
			(
				SELECT COUNT(*) FROM chat_analytics.message_facts mf3
				WHERE mf3.chat_id = mf.chat_id 
				AND mf3.id > mf.id
				AND (mf3.sender_id = mf.recipient_id OR mf3.recipient_id = mf.sender_id)
				LIMIT 10
			) AS follow_up_messages
		FROM chat_analytics.message_facts mf
		WHERE mf.id IN (
			SELECT MAX(id) FROM chat_analytics.message_facts
			GROUP BY sender_id, recipient_id, chat_id
		)
		ORDER BY mf.sender_id, mf.recipient_id
	`)
	if err != nil {
		return nil, fmt.Errorf("ошибка при запросе данных сообщений: %w", err)
	}
	defer rows.Close()

	// Количество обработанных записей
	count := 0

	// Обрабатываем результаты запроса
	for rows.Next() {
		var (
			senderID, recipientID, messageLength int
			responseTime                         float64
			hasResponse                          bool
			followUpMessages                     int
		)

		// Сканируем строку результата
		err := rows.Scan(
			&senderID,
			&recipientID,
			&messageLength,
			&responseTime,
			&hasResponse,
			&followUpMessages,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка при сканировании результатов: %w", err)
		}

		// Инициализируем вложенную карту, если её еще нет
		if _, exists := messagesMap[senderID]; !exists {
			messagesMap[senderID] = make(map[int][]MessageInfo)
		}

		// Добавляем информацию о сообщении
		messagesMap[senderID][recipientID] = append(
			messagesMap[senderID][recipientID],
			MessageInfo{
				ResponseTimeMinutes: responseTime,
				MessageLength:       messageLength,
				HasResponse:         hasResponse,
				FollowUpMessages:    followUpMessages,
			},
		)

		count++
	}

	// Проверяем ошибки после цикла
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке результатов: %w", err)
	}

	// Если данных нет, получаем данные из таблиц chat_facts для агрегированной информации
	if count == 0 {
		s.logger.Info("Данные сообщений не найдены в message_facts, используем данные из chat_facts")
		return s.getMessagesFromChatFacts()
	}

	s.logger.Info("Получено %d записей о сообщениях для %d отправителей",
		count, len(messagesMap))
	return messagesMap, nil
}

// getMessagesFromChatFacts получает данные из таблицы chat_facts
// Используется как запасной вариант, если нет детальных данных
func (s *MySQLDataService) getMessagesFromChatFacts() (map[int]map[int][]MessageInfo, error) {
	// Результирующая структура данных
	messagesMap := make(map[int]map[int][]MessageInfo)

	// Запрос данных из таблицы chat_facts
	rows, err := s.olapDB.Query(`
		SELECT 
			cf.buyer_id,
			cf.seller_id,
			cf.total_messages,
			cf.avg_message_length,
			cf.avg_response_time_minutes
		FROM chat_analytics.chat_facts cf
		ORDER BY cf.buyer_id, cf.seller_id
	`)
	if err != nil {
		return nil, fmt.Errorf("ошибка при запросе данных из chat_facts: %w", err)
	}
	defer rows.Close()

	// Обрабатываем результаты запроса
	for rows.Next() {
		var (
			buyerID, sellerID, totalMessages  int
			avgMessageLength, avgResponseTime float64
		)

		// Сканируем строку результата
		err := rows.Scan(
			&buyerID,
			&sellerID,
			&totalMessages,
			&avgMessageLength,
			&avgResponseTime,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка при сканировании результатов chat_facts: %w", err)
		}

		// Создаем записи для обоих направлений (покупатель -> продавец и продавец -> покупатель)
		// с половиной сообщений в каждом направлении

		// 1. Направление покупатель -> продавец
		if _, exists := messagesMap[buyerID]; !exists {
			messagesMap[buyerID] = make(map[int][]MessageInfo)
		}
		// Добавляем агрегированную информацию как одну запись с "усредненными" данными
		messagesMap[buyerID][sellerID] = []MessageInfo{
			{
				ResponseTimeMinutes: avgResponseTime,
				MessageLength:       int(avgMessageLength),
				HasResponse:         true,              // Предполагаем, что в чатах есть взаимодействие
				FollowUpMessages:    totalMessages / 2, // Половина общего числа сообщений
			},
		}

		// 2. Направление продавец -> покупатель
		if _, exists := messagesMap[sellerID]; !exists {
			messagesMap[sellerID] = make(map[int][]MessageInfo)
		}
		// То же самое для обратного направления
		messagesMap[sellerID][buyerID] = []MessageInfo{
			{
				ResponseTimeMinutes: avgResponseTime,
				MessageLength:       int(avgMessageLength),
				HasResponse:         true,
				FollowUpMessages:    totalMessages / 2,
			},
		}
	}

	// Проверяем ошибки после цикла
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке результатов chat_facts: %w", err)
	}

	s.logger.Info("Получены агрегированные данные из chat_facts для %d пользователей",
		len(messagesMap))
	return messagesMap, nil
}

// GetUserCount возвращает количество пользователей в системе
func (s *MySQLDataService) GetUserCount() (int, error) {
	var count int
	err := s.olapDB.QueryRow("SELECT COUNT(*) FROM chat_analytics.user_dimension").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("ошибка при подсчете пользователей: %w", err)
	}
	return count, nil
}
