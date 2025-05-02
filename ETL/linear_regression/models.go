package linear_regression

import (
	"time"
)

// DataPoint представляет точку данных для линейной регрессии
type DataPoint struct {
	X    float64   // Порядковый номер дня (относительно начала периода)
	Y    float64   // Количество сообщений в день
	Date time.Time // Фактическая дата
}

// RegressionResult содержит результаты линейной регрессии
type RegressionResult struct {
	A           float64     // Коэффициент наклона
	B           float64     // Сдвиг
	R           float64     // Коэффициент корреляции Пирсона
	R2          float64     // Коэффициент детерминации
	PeriodStart time.Time   // Начало анализируемого периода
	PeriodEnd   time.Time   // Конец анализируемого периода
	DataPoints  []DataPoint // Исходные точки данных
}

// ForecastPoint представляет точку прогноза
type ForecastPoint struct {
	Date          time.Time // Дата прогноза
	ForecastValue float64   // Прогнозируемое значение
	CILower       float64   // Нижняя граница доверительного интервала
	CIUpper       float64   // Верхняя граница доверительного интервала
}

// PredictionRepository интерфейс для работы с хранилищем прогнозов
type PredictionRepository interface {
	// SavePrediction сохраняет прогноз активности в БД
	SavePrediction(result RegressionResult, forecast ForecastPoint) error

	// GetForecasts получает прогнозы для указанного периода
	GetForecasts(startDate, endDate time.Time) ([]ForecastPoint, error)

	// GetLastRegressionResult получает последний результат регрессии
	GetLastRegressionResult() (*RegressionResult, error)

	// DeleteOldPredictions удаляет устаревшие прогнозы (старше N дней)
	DeleteOldPredictions(olderThan time.Time) error
}
