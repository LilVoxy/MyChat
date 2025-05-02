package linear_regression

import (
	"fmt"
	"math"
)

// RoundToThousandth округляет число до тысячных (3 знака после запятой)
func RoundToThousandth(value float64) float64 {
	return math.Round(value*1000) / 1000
}

// LinearRegression выполняет расчет линейной регрессии на основе входных данных
// и возвращает результат с коэффициентами
func LinearRegression(points []DataPoint) (*RegressionResult, error) {
	if len(points) < 2 {
		return nil, fmt.Errorf("для расчета линейной регрессии требуется минимум 2 точки, получено: %d", len(points))
	}

	// Находим минимальную и максимальную даты
	minDate := points[0].Date
	maxDate := points[0].Date
	for _, p := range points {
		if p.Date.Before(minDate) {
			minDate = p.Date
		}
		if p.Date.After(maxDate) {
			maxDate = p.Date
		}
	}

	// Расчет коэффициентов линейной регрессии методом наименьших квадратов
	// формулы:
	// a = (n*sum(x*y) - sum(x)*sum(y)) / (n*sum(x^2) - (sum(x))^2)
	// b = (sum(y) - a*sum(x)) / n
	n := float64(len(points))
	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0
	sumY2 := 0.0

	for _, p := range points {
		sumX += p.X
		sumY += p.Y
		sumXY += p.X * p.Y
		sumX2 += p.X * p.X
		sumY2 += p.Y * p.Y
	}

	// Расчет коэффициента наклона (a)
	denominator := n*sumX2 - sumX*sumX
	if math.Abs(denominator) < 1e-10 {
		return nil, fmt.Errorf("все X одинаковы, невозможно вычислить наклон")
	}

	a := (n*sumXY - sumX*sumY) / denominator

	// Расчет сдвига (b)
	b := (sumY - a*sumX) / n

	// Расчет коэффициента корреляции Пирсона (r)
	// r = (n*sum(x*y) - sum(x)*sum(y)) / sqrt[(n*sum(x^2) - (sum(x))^2) * (n*sum(y^2) - (sum(y))^2)]
	numerator := n*sumXY - sumX*sumY
	denominator = math.Sqrt((n*sumX2 - sumX*sumX) * (n*sumY2 - sumY*sumY))

	var r float64
	if math.Abs(denominator) < 1e-10 {
		r = 0 // нет корреляции или все значения одинаковы
	} else {
		r = numerator / denominator
	}

	// Коэффициент детерминации (r^2)
	r2 := r * r

	// Округляем все результаты до тысячных (3 знака после запятой)
	a = RoundToThousandth(a)
	b = RoundToThousandth(b)
	r = RoundToThousandth(r)
	r2 = RoundToThousandth(r2)

	return &RegressionResult{
		A:           a,
		B:           b,
		R:           r,
		R2:          r2,
		PeriodStart: minDate,
		PeriodEnd:   maxDate,
		DataPoints:  points,
	}, nil
}

// Predict прогнозирует значение Y для заданного X на основе модели линейной регрессии
func Predict(result *RegressionResult, x float64) float64 {
	// Расчет прогноза и округление до тысячных
	return RoundToThousandth(result.A*x + result.B)
}

// CalculateConfidenceInterval вычисляет доверительный интервал для прогноза
// на основе стандартной ошибки и уровня значимости
func CalculateConfidenceInterval(result *RegressionResult, x float64, confidenceLevel float64) (float64, float64) {
	// Стандартная ошибка оценки
	n := float64(len(result.DataPoints))

	// Средние значения
	meanX := 0.0
	for _, p := range result.DataPoints {
		meanX += p.X
	}
	meanX /= n

	// Сумма квадратов отклонений
	sumSqDevX := 0.0
	sumSqResiduals := 0.0

	for _, p := range result.DataPoints {
		predY := Predict(result, p.X)
		sumSqDevX += (p.X - meanX) * (p.X - meanX)
		sumSqResiduals += (p.Y - predY) * (p.Y - predY)
	}

	// Стандартная ошибка оценки
	standardError := math.Sqrt(sumSqResiduals / (n - 2))

	// Значение t-статистики для 95% доверительного интервала
	// Для простоты используем приближение t ≈ 2 для большинства случаев (n > 30)
	// В реальной ситуации следует использовать таблицу распределения Стьюдента
	tStat := 2.0
	if confidenceLevel == 0.99 {
		tStat = 2.58 // Для 99% доверительного интервала
	} else if confidenceLevel == 0.90 {
		tStat = 1.64 // Для 90% доверительного интервала
	}

	// Расчет стандартной ошибки прогноза
	// Стандартная ошибка прогноза включает ошибку регрессии и ошибку предсказания
	predictionStdError := standardError * math.Sqrt(1+1/n+(x-meanX)*(x-meanX)/sumSqDevX)

	// Расчет границ доверительного интервала
	margin := tStat * predictionStdError
	yPred := Predict(result, x)

	// Округляем значения доверительного интервала до тысячных
	return RoundToThousandth(yPred - margin), RoundToThousandth(yPred + margin)
}

// GenerateForecasts генерирует прогнозы на указанное количество дней вперед
func GenerateForecasts(result *RegressionResult, daysAhead int, confidenceLevel float64) []ForecastPoint {
	forecasts := make([]ForecastPoint, daysAhead)

	// Определяем базовую дату, от которой будем генерировать прогнозы
	// Обычно это последняя дата в наборе данных
	lastDate := result.PeriodEnd

	// Максимальный X в наборе данных
	maxX := 0.0
	for _, p := range result.DataPoints {
		if p.X > maxX {
			maxX = p.X
		}
	}

	// Генерация прогнозов на будущие дни
	for i := 0; i < daysAhead; i++ {
		// X для прогноза: maxX + (i+1) дней
		x := maxX + float64(i+1)

		// Прогнозируемое значение (уже округляется в функции Predict)
		yPred := Predict(result, x)

		// Доверительный интервал (уже округляется в функции CalculateConfidenceInterval)
		lower, upper := CalculateConfidenceInterval(result, x, confidenceLevel)

		// Дата прогноза
		forecastDate := lastDate.AddDate(0, 0, i+1)

		forecasts[i] = ForecastPoint{
			Date:          forecastDate,
			ForecastValue: yPred,
			CILower:       lower,
			CIUpper:       upper,
		}
	}

	return forecasts
}
