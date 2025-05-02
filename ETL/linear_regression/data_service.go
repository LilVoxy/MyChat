package linear_regression

import (
	"database/sql"
	"fmt"
	"time"
)

// DataService сервис для получения данных из OLAP-куба
type DataService struct {
	db *sql.DB
}

// NewDataService создает новый сервис для работы с данными
func NewDataService(db *sql.DB) *DataService {
	return &DataService{
		db: db,
	}
}

// GetDailyActivityData получает данные о ежедневной активности за указанный период
func (s *DataService) GetDailyActivityData(daysBack int) ([]DataPoint, error) {
	// Получаем данные из таблицы daily_activity_facts
	// для указанного периода времени
	query := `
	SELECT 
		td.full_date, 
		daf.total_messages
	FROM 
		chat_analytics.daily_activity_facts daf
	JOIN 
		chat_analytics.time_dimension td ON daf.date_id = td.id
	WHERE 
		td.full_date >= DATE_SUB(CURDATE(), INTERVAL ? DAY)
	ORDER BY 
		td.full_date;`

	rows, err := s.db.Query(query, daysBack)
	if err != nil {
		return nil, fmt.Errorf("ошибка при выполнении запроса к OLAP: %w", err)
	}
	defer rows.Close()

	var dataPoints []DataPoint
	var baseDate time.Time
	var firstPoint bool = true

	for rows.Next() {
		var date time.Time
		var messages int

		if err := rows.Scan(&date, &messages); err != nil {
			return nil, fmt.Errorf("ошибка при чтении данных: %w", err)
		}

		if firstPoint {
			baseDate = date
			firstPoint = false
		}

		// Рассчитываем X как количество дней от начала периода
		days := date.Sub(baseDate).Hours() / 24

		dataPoints = append(dataPoints, DataPoint{
			X:    days,
			Y:    float64(messages),
			Date: date,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при итерации по результатам: %w", err)
	}

	if len(dataPoints) == 0 {
		return nil, fmt.Errorf("не найдены данные о ежедневной активности за последние %d дней", daysBack)
	}

	return dataPoints, nil
}

// GetHourlyActivityData получает данные о почасовой активности за указанный период
func (s *DataService) GetHourlyActivityData(hoursBack int) ([]DataPoint, error) {
	// Получаем данные из таблицы hourly_activity_facts
	// для указанного периода времени
	query := `
	SELECT 
		CONCAT(td.full_date, ' ', LPAD(haf.hour_of_day, 2, '0'), ':00:00') as datetime, 
		haf.total_messages
	FROM 
		chat_analytics.hourly_activity_facts haf
	JOIN 
		chat_analytics.time_dimension td ON haf.date_id = td.id
	WHERE 
		CONCAT(td.full_date, ' ', LPAD(haf.hour_of_day, 2, '0'), ':00:00') >= 
		DATE_SUB(NOW(), INTERVAL ? HOUR)
	ORDER BY 
		td.full_date, haf.hour_of_day;`

	rows, err := s.db.Query(query, hoursBack)
	if err != nil {
		return nil, fmt.Errorf("ошибка при выполнении запроса к OLAP: %w", err)
	}
	defer rows.Close()

	var dataPoints []DataPoint
	var baseTime time.Time
	var firstPoint bool = true

	for rows.Next() {
		var datetime string
		var messages int

		if err := rows.Scan(&datetime, &messages); err != nil {
			return nil, fmt.Errorf("ошибка при чтении данных: %w", err)
		}

		// Преобразуем строку даты/времени обратно в time.Time
		timestamp, err := time.Parse("2006-01-02 15:04:05", datetime)
		if err != nil {
			return nil, fmt.Errorf("ошибка при парсинге времени: %w", err)
		}

		if firstPoint {
			baseTime = timestamp
			firstPoint = false
		}

		// Рассчитываем X как количество часов от начала периода
		hours := timestamp.Sub(baseTime).Hours()

		dataPoints = append(dataPoints, DataPoint{
			X:    hours,
			Y:    float64(messages),
			Date: timestamp,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при итерации по результатам: %w", err)
	}

	if len(dataPoints) == 0 {
		return nil, fmt.Errorf("не найдены данные о почасовой активности за последние %d часов", hoursBack)
	}

	return dataPoints, nil
}
