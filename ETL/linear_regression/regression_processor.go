package linear_regression

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// Config конфигурация процессора линейной регрессии
type Config struct {
	// Количество дней для анализа
	AnalysisPeriodDays int
	// Количество дней для прогноза
	ForecastDays int
	// Уровень доверия (0.90, 0.95, 0.99)
	ConfidenceLevel float64
	// Минимальное значение r² для признания модели значимой
	MinR2Threshold float64
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig() Config {
	return Config{
		AnalysisPeriodDays: 30,
		ForecastDays:       14,
		ConfidenceLevel:    0.95,
		MinR2Threshold:     0.30, // 30% объяснённой вариации
	}
}

// RegressionProcessor процессор линейной регрессии
type RegressionProcessor struct {
	dataService *DataService
	repository  *MySQLPredictionRepository
	logger      *utils.ETLLogger
	config      Config
}

// NewRegressionProcessor создает новый процессор линейной регрессии
func NewRegressionProcessor(
	dataService *DataService,
	repository *MySQLPredictionRepository,
	logger *utils.ETLLogger,
	config Config,
) *RegressionProcessor {
	return &RegressionProcessor{
		dataService: dataService,
		repository:  repository,
		logger:      logger,
		config:      config,
	}
}

// Process выполняет основной процесс: анализ данных, построение модели и сохранение прогнозов
func (p *RegressionProcessor) Process() error {
	startTime := time.Now()
	p.logger.Info("Запуск процесса линейной регрессии для прогнозирования активности")

	// 1. Убеждаемся, что таблица существует
	if err := p.repository.EnsureTableExists(); err != nil {
		return fmt.Errorf("ошибка при проверке/создании таблицы: %w", err)
	}

	// 2. Получаем данные для анализа
	p.logger.Info("Получение данных о ежедневной активности за последние %d дней", p.config.AnalysisPeriodDays)
	dataPoints, err := p.dataService.GetDailyActivityData(p.config.AnalysisPeriodDays)
	if err != nil {
		return fmt.Errorf("ошибка при получении данных: %w", err)
	}

	p.logger.Info("Получено %d точек данных для анализа", len(dataPoints))

	// 3. Строим модель линейной регрессии
	p.logger.Info("Построение модели линейной регрессии (все значения округляются до тысячных)")
	regressionResult, err := LinearRegression(dataPoints)
	if err != nil {
		return fmt.Errorf("ошибка при построении модели линейной регрессии: %w", err)
	}

	// 4. Оцениваем качество модели
	p.logger.Info("Результаты модели: коэффициент наклона (a)=%.3f, сдвиг (b)=%.3f, R=%.3f, R²=%.3f",
		regressionResult.A, regressionResult.B, regressionResult.R, regressionResult.R2)

	// Логируем информацию о периоде анализа
	p.logger.Info("Период анализа: с %v по %v",
		regressionResult.PeriodStart.Format("2006-01-02"),
		regressionResult.PeriodEnd.Format("2006-01-02"))

	// Если модель недостаточно хороша, логируем предупреждение
	if regressionResult.R2 < p.config.MinR2Threshold {
		p.logger.Info("Низкое качество модели (R²=%.3f < %.3f). Однако прогноз будет сделан.",
			regressionResult.R2, p.config.MinR2Threshold)
	}

	// 5. Генерируем прогнозы
	p.logger.Info("Генерация прогнозов на %d дней вперед от %v",
		p.config.ForecastDays,
		regressionResult.PeriodEnd.Format("2006-01-02"))
	forecasts := GenerateForecasts(regressionResult, p.config.ForecastDays, p.config.ConfidenceLevel)

	// 6. Сохраняем прогнозы в БД
	p.logger.Info("Сохранение %d прогнозов в базу данных", len(forecasts))
	if err := p.repository.SaveMultiplePredictions(*regressionResult, forecasts); err != nil {
		return fmt.Errorf("ошибка при сохранении прогнозов: %w", err)
	}

	// 7. Удаляем устаревшие прогнозы (старше 90 дней)
	deleteOlderThan := time.Now().AddDate(0, 0, -90)
	if err := p.repository.DeleteOldPredictions(deleteOlderThan); err != nil {
		// Это некритическая ошибка, просто логируем
		p.logger.Info("Не удалось удалить устаревшие прогнозы: %v", err)
	}

	executionTime := time.Since(startTime)
	p.logger.Info("Процесс линейной регрессии успешно завершен. Время выполнения: %v", executionTime)
	return nil
}

// RunAsPartOfETL запускает процесс как часть ETL
func RunAsPartOfETL(olapDB *sql.DB, logger *utils.ETLLogger) error {
	return RunWithCustomConfig(olapDB, logger, DefaultConfig())
}

// RunWithCustomConfig запускает процесс с пользовательской конфигурацией
func RunWithCustomConfig(olapDB *sql.DB, logger *utils.ETLLogger, config Config) error {
	log.Println("Запуск процесса линейной регрессии с пользовательской конфигурацией")

	// Инициализируем сервисы
	dataService := NewDataService(olapDB)
	repository := NewMySQLPredictionRepository(olapDB)
	processor := NewRegressionProcessor(dataService, repository, logger, config)

	// Запускаем процесс
	return processor.Process()
}
