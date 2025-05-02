package linear_regression

import (
	"database/sql"
	"fmt"
	"time"
)

// MySQLPredictionRepository реализация PredictionRepository для работы с MySQL
type MySQLPredictionRepository struct {
	db *sql.DB
}

// NewMySQLPredictionRepository создает новый репозиторий для работы с прогнозами
func NewMySQLPredictionRepository(db *sql.DB) *MySQLPredictionRepository {
	return &MySQLPredictionRepository{
		db: db,
	}
}

// EnsureTableExists проверяет наличие таблицы и создает ее при необходимости
func (r *MySQLPredictionRepository) EnsureTableExists() error {
	query := `
	CREATE TABLE IF NOT EXISTS chat_analytics.activity_trend_predictions (
		id INT AUTO_INCREMENT PRIMARY KEY,
		period_start DATE NOT NULL,
		period_end DATE NOT NULL,
		a DOUBLE NOT NULL,
		b DOUBLE NOT NULL,
		r DOUBLE NOT NULL,
		r2 DOUBLE NOT NULL,
		forecast_date DATE NOT NULL,
		forecast_value DOUBLE NOT NULL,
		ci_lower DOUBLE NOT NULL,
		ci_upper DOUBLE NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_forecast_date (forecast_date),
		INDEX idx_period (period_start, period_end)
	);`

	_, err := r.db.Exec(query)
	return err
}

// SavePrediction сохраняет прогноз в БД
func (r *MySQLPredictionRepository) SavePrediction(result RegressionResult, forecast ForecastPoint) error {
	query := `
	INSERT INTO chat_analytics.activity_trend_predictions
		(period_start, period_end, a, b, r, r2, forecast_date, forecast_value, ci_lower, ci_upper)
	VALUES
		(?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

	_, err := r.db.Exec(
		query,
		result.PeriodStart,
		result.PeriodEnd,
		result.A,
		result.B,
		result.R,
		result.R2,
		forecast.Date,
		forecast.ForecastValue,
		forecast.CILower,
		forecast.CIUpper,
	)

	return err
}

// SaveMultiplePredictions сохраняет несколько прогнозов в транзакции
func (r *MySQLPredictionRepository) SaveMultiplePredictions(result RegressionResult, forecasts []ForecastPoint) error {
	// Начинаем транзакцию
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("не удалось начать транзакцию: %w", err)
	}

	// Подготавливаем запрос
	query := `
	INSERT INTO chat_analytics.activity_trend_predictions
		(period_start, period_end, a, b, r, r2, forecast_date, forecast_value, ci_lower, ci_upper)
	VALUES
		(?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

	stmt, err := tx.Prepare(query)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("не удалось подготовить запрос: %w", err)
	}
	defer stmt.Close()

	// Выполняем запрос для каждого прогноза
	for _, forecast := range forecasts {
		_, err := stmt.Exec(
			result.PeriodStart,
			result.PeriodEnd,
			result.A,
			result.B,
			result.R,
			result.R2,
			forecast.Date,
			forecast.ForecastValue,
			forecast.CILower,
			forecast.CIUpper,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("не удалось выполнить запрос: %w", err)
		}
	}

	// Фиксируем транзакцию
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("не удалось зафиксировать транзакцию: %w", err)
	}

	return nil
}

// GetForecasts получает прогнозы для указанного периода
func (r *MySQLPredictionRepository) GetForecasts(startDate, endDate time.Time) ([]ForecastPoint, error) {
	query := `
	SELECT forecast_date, forecast_value, ci_lower, ci_upper
	FROM chat_analytics.activity_trend_predictions
	WHERE forecast_date BETWEEN ? AND ?
	ORDER BY forecast_date;`

	rows, err := r.db.Query(query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("ошибка при выполнении запроса: %w", err)
	}
	defer rows.Close()

	var forecasts []ForecastPoint
	for rows.Next() {
		var f ForecastPoint
		if err := rows.Scan(&f.Date, &f.ForecastValue, &f.CILower, &f.CIUpper); err != nil {
			return nil, fmt.Errorf("ошибка при чтении данных: %w", err)
		}
		forecasts = append(forecasts, f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при итерации по результатам: %w", err)
	}

	return forecasts, nil
}

// GetLastRegressionResult получает последний результат регрессии
func (r *MySQLPredictionRepository) GetLastRegressionResult() (*RegressionResult, error) {
	query := `
	SELECT a, b, r, r2, period_start, period_end
	FROM chat_analytics.activity_trend_predictions
	ORDER BY created_at DESC
	LIMIT 1;`

	var result RegressionResult
	err := r.db.QueryRow(query).Scan(
		&result.A,
		&result.B,
		&result.R,
		&result.R2,
		&result.PeriodStart,
		&result.PeriodEnd,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Нет данных - возвращаем nil
	}

	if err != nil {
		return nil, fmt.Errorf("ошибка при получении последнего результата регрессии: %w", err)
	}

	return &result, nil
}

// DeleteOldPredictions удаляет устаревшие прогнозы
func (r *MySQLPredictionRepository) DeleteOldPredictions(olderThan time.Time) error {
	query := `
	DELETE FROM chat_analytics.activity_trend_predictions
	WHERE created_at < ?;`

	_, err := r.db.Exec(query, olderThan)
	if err != nil {
		return fmt.Errorf("ошибка при удалении устаревших прогнозов: %w", err)
	}

	return nil
}
