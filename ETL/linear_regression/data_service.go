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
	// Сначала определим последнюю доступную дату в таблице
	lastDateQuery := `
	SELECT 
		MAX(td.full_date)
	FROM 
		chat_analytics.daily_activity_facts daf
	JOIN 
		chat_analytics.time_dimension td ON daf.date_id = td.id;`

	var lastDate time.Time
	err := s.db.QueryRow(lastDateQuery).Scan(&lastDate)
	if err != nil {
		return nil, fmt.Errorf("ошибка при определении последней даты: %w", err)
	}

	// Получаем данные из таблицы daily_activity_facts
	// для указанного периода времени относительно последней даты
	query := `
	SELECT 
		td.full_date, 
		daf.total_messages
	FROM 
		chat_analytics.daily_activity_facts daf
	JOIN 
		chat_analytics.time_dimension td ON daf.date_id = td.id
	WHERE 
		td.full_date >= DATE_SUB(?, INTERVAL ? DAY)
		AND td.full_date <= ?
	ORDER BY 
		td.full_date;`

	rows, err := s.db.Query(query, lastDate, daysBack, lastDate)
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
		return nil, fmt.Errorf("не найдены данные о ежедневной активности за последние %d дней от %v", daysBack, lastDate)
	}

	return dataPoints, nil
}
