package transform

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// TimeDimensionProcessor отвечает за обработку измерения времени
type TimeDimensionProcessor struct {
	oltpDB *sql.DB
	olapDB *sql.DB
	logger *utils.ETLLogger
}

// NewTimeDimensionProcessor создает новый экземпляр TimeDimensionProcessor
func NewTimeDimensionProcessor(oltpDB, olapDB *sql.DB, logger *utils.ETLLogger) *TimeDimensionProcessor {
	return &TimeDimensionProcessor{
		oltpDB: oltpDB,
		olapDB: olapDB,
		logger: logger,
	}
}

// EnsureTimeDimension проверяет и при необходимости обновляет измерение времени
// В нашем случае, измерение времени уже создано с 22.04.2025 по 21.04.2026
// Эта функция просто проверяет наличие измерения и его полноту
func (p *TimeDimensionProcessor) EnsureTimeDimension() error {
	p.logger.Debug("Проверка измерения времени...")

	// Проверяем, есть ли записи в time_dimension
	var count int
	err := p.olapDB.QueryRow("SELECT COUNT(*) FROM chat_analytics.time_dimension").Scan(&count)
	if err != nil {
		return fmt.Errorf("ошибка при проверке измерения времени: %w", err)
	}

	if count == 0 {
		p.logger.Debug("Измерение времени пусто, создаем записи...")
		return p.createInitialTimeDimension()
	}

	p.logger.Debug("Измерение времени содержит %d записей", count)
	return nil
}

// createInitialTimeDimension создает начальные записи в измерении времени
// Обычно это делается один раз при инициализации OLAP-куба
func (p *TimeDimensionProcessor) createInitialTimeDimension() error {
	p.logger.Info("Создание измерения времени...")

	// Определяем начальную и конечную даты (22.04.2025 - 21.04.2026)
	startDate := time.Date(2025, 4, 22, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 4, 21, 23, 0, 0, 0, time.UTC)

	// Подготавливаем SQL-запрос для вставки
	stmt, err := p.olapDB.Prepare(`
		INSERT INTO chat_analytics.time_dimension 
		(full_date, year, quarter, month, month_name, week_of_year, 
		day_of_month, day_of_week, day_name, is_weekend, hour_of_day) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("ошибка при подготовке запроса: %w", err)
	}
	defer stmt.Close()

	// Массивы для названий месяцев и дней недели
	monthNames := []string{"January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December"}
	dayNames := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

	// Итерируемся по всем часам в указанном периоде
	current := startDate
	count := 0
	for current.Before(endDate) || current.Equal(endDate) {
		// Получаем компоненты даты
		year := current.Year()
		month := int(current.Month())
		monthName := monthNames[month-1]

		// Определяем квартал
		quarter := (month-1)/3 + 1

		// Номер недели в году (приблизительно)
		yearDay := current.YearDay()
		weekOfYear := (yearDay-1)/7 + 1

		dayOfMonth := current.Day()
		dayOfWeek := int(current.Weekday()) + 1 // 1=Sunday, 7=Saturday
		dayName := dayNames[dayOfWeek-1]

		// Выходной день (суббота или воскресенье)
		isWeekend := dayOfWeek == 1 || dayOfWeek == 7

		hourOfDay := current.Hour()

		// Вставляем запись
		_, err := stmt.Exec(
			current.Format("2006-01-02"), // full_date
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
			return fmt.Errorf("ошибка при вставке записи измерения времени: %w", err)
		}

		// Переходим к следующему часу
		current = current.Add(time.Hour)
		count++

		// Логируем прогресс каждые 24 часа
		if count%24 == 0 {
			p.logger.Debug("Создано %d записей измерения времени...", count)
		}
	}

	p.logger.Info("Измерение времени создано. Всего записей: %d", count)
	return nil
}
