package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/config"
	"github.com/LilVoxy/coursework_chat/ETL/extractors"
	"github.com/LilVoxy/coursework_chat/ETL/linear_regression"
	"github.com/LilVoxy/coursework_chat/ETL/load"
	"github.com/LilVoxy/coursework_chat/ETL/models"
	"github.com/LilVoxy/coursework_chat/ETL/transform"
	"github.com/LilVoxy/coursework_chat/ETL/utils"
	"github.com/go-co-op/gocron"
)

type ETLRunner struct {
	config          config.ETLConfig
	dbConnections   *config.DBConnections
	logger          *utils.ETLLogger
	extractor       *extractors.Extractor
	transformer     *transform.Transformer
	loadManager     *load.LoadManager
	etlLogRepo      models.ETLLogRepository
	lastRunMetadata models.ETLMetadata
}

// NewETLRunner создает новый экземпляр ETLRunner
func NewETLRunner() (*ETLRunner, error) {
	// Получаем конфигурацию
	etlConfig := config.GetConfig()

	// Инициализируем логгер
	logger := utils.NewETLLogger(etlConfig.EnableDetailedLogging)
	logger.Info("Инициализация ETL Runner")

	// Подключаемся к базам данных
	connections, err := config.ConnectDatabases(etlConfig)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к базам данных: %w", err)
	}

	// Инициализируем репозиторий логов ETL
	etlLogRepo := models.NewMySQLETLLogRepository(connections.OLAPDB)

	// Создаем таблицу логов, если она еще не существует
	if err := etlLogRepo.CreateETLLogTable(); err != nil {
		return nil, fmt.Errorf("ошибка при создании таблицы логов ETL: %w", err)
	}

	// Создаем экстрактор
	extractor := extractors.NewExtractor(connections.OLTPDB, logger, etlConfig.BatchSize)

	// Создаем трансформатор
	transformer := transform.NewTransformer(connections.OLTPDB, connections.OLAPDB, logger)

	// Создаем загрузчик
	loadManager := load.NewLoadManager(connections.OLAPDB, logger)

	return &ETLRunner{
		config:        etlConfig,
		dbConnections: connections,
		logger:        logger,
		extractor:     extractor,
		transformer:   transformer,
		loadManager:   loadManager,
		etlLogRepo:    etlLogRepo,
	}, nil
}

// Close закрывает соединения с базами данных
func (r *ETLRunner) Close() {
	r.logger.Info("Завершение работы ETL Runner")
	config.CloseDatabases(r.dbConnections)
}

// ExecuteETL выполняет полный ETL процесс
func (r *ETLRunner) ExecuteETL() error {
	r.logger.Info("Запуск ETL процесса")
	startTime := time.Now()

	// Создаем запись в журнале ETL
	logID, err := r.etlLogRepo.CreateLogEntry(startTime)
	if err != nil {
		r.logger.Error("Ошибка при создании записи в журнале ETL: %v", err)
		return fmt.Errorf("ошибка при создании записи в журнале ETL: %w", err)
	}

	// Инициализируем лог запуска ETL
	runLog := &models.ETLRunLog{
		ID:        logID,
		StartTime: startTime,
		Status:    "in_progress",
	}

	// Получаем метаданные последнего успешного запуска
	lastRun, err := r.etlLogRepo.GetLastSuccessfulRun()
	if err != nil {
		r.logger.Error("Не удалось получить информацию о последнем успешном запуске: %v. Будет выполнен полный ETL.", err)
		// Продолжаем выполнение, но обрабатываем все данные
	}

	var lastRunTime time.Time
	var lastProcessedMessageID int

	if lastRun != nil {
		lastRunTime = lastRun.EndTime
		lastProcessedMessageID = lastRun.LastProcessedMessageID
		r.logger.Info("Последний успешный запуск: %v, ID последнего сообщения: %d", lastRunTime, lastProcessedMessageID)
	}

	// 1. Фаза извлечения данных (Extract)
	extractedData, err := r.extractor.Extract(lastRunTime, lastProcessedMessageID)
	if err != nil {
		errMsg := fmt.Sprintf("Ошибка в фазе Extract: %v", err)
		r.logger.Error(errMsg)
		r.updateETLRunLogFailure(runLog, errMsg)
		return fmt.Errorf("ошибка в фазе Extract: %w", err)
	}

	// Если нет новых данных, завершаем процесс
	if len(extractedData.Messages) == 0 && len(extractedData.Users) == 0 && len(extractedData.Chats) == 0 {
		r.logger.Info("Нет новых данных для обработки")
		r.updateETLRunLogSuccess(runLog, 0, 0, 0, lastProcessedMessageID)
		return nil
	}

	// Определяем ID последнего обработанного сообщения
	var maxMessageID int
	for _, msg := range extractedData.Messages {
		if msg.ID > maxMessageID {
			maxMessageID = msg.ID
		}
	}

	if maxMessageID == 0 {
		maxMessageID = lastProcessedMessageID
	}

	// 2. Фаза трансформации данных (Transform)
	transformedData, err := r.transformer.Transform(extractedData)
	if err != nil {
		errMsg := fmt.Sprintf("Ошибка в фазе Transform: %v", err)
		r.logger.Error(errMsg)
		r.updateETLRunLogFailure(runLog, errMsg)
		return fmt.Errorf("ошибка в фазе Transform: %w", err)
	}

	// 3. Фаза загрузки данных (Load)
	err = r.loadManager.Load(transformedData)
	if err != nil {
		errMsg := fmt.Sprintf("Ошибка в фазе Load: %v", err)
		r.logger.Error(errMsg)
		r.updateETLRunLogFailure(runLog, errMsg)
		return fmt.Errorf("ошибка в фазе Load: %w", err)
	}

	// 4. Запускаем линейную регрессию для прогнозирования трендов активности
	r.logger.Info("Запуск линейной регрессии для прогнозирования трендов активности")
	if err := r.runLinearRegression(); err != nil {
		r.logger.Error("Ошибка при выполнении линейной регрессии: %v", err)
		// Не прерываем ETL процесс из-за ошибки в линейной регрессии
		// Это некритичный компонент
	}

	// Обновляем запись в журнале с информацией об успешном выполнении
	r.updateETLRunLogSuccess(runLog,
		len(extractedData.Users),
		len(extractedData.Chats),
		len(extractedData.Messages),
		maxMessageID)

	r.logger.Info("ETL процесс успешно завершен. Длительность: %v", time.Since(startTime))
	return nil
}

// updateETLRunLogSuccess обновляет запись в журнале ETL при успешном завершении
func (r *ETLRunner) updateETLRunLogSuccess(runLog *models.ETLRunLog, usersProcessed, chatsProcessed, messagesProcessed, lastMessageID int) {
	runLog.EndTime = time.Now()
	runLog.Status = "success"
	runLog.UsersProcessed = usersProcessed
	runLog.ChatsProcessed = chatsProcessed
	runLog.MessagesProcessed = messagesProcessed
	runLog.LastProcessedMessageID = lastMessageID

	if err := r.etlLogRepo.UpdateLogEntrySuccess(
		runLog.ID,
		runLog.EndTime,
		runLog.UsersProcessed,
		runLog.ChatsProcessed,
		runLog.MessagesProcessed,
		runLog.LastProcessedMessageID); err != nil {
		r.logger.Error("Ошибка при обновлении записи в журнале ETL: %v", err)
	}
}

// updateETLRunLogFailure обновляет запись в журнале ETL при ошибке
func (r *ETLRunner) updateETLRunLogFailure(runLog *models.ETLRunLog, errorMessage string) {
	runLog.EndTime = time.Now()
	runLog.Status = "failed"
	runLog.ErrorMessage = errorMessage

	if err := r.etlLogRepo.UpdateLogEntryFailure(
		runLog.ID,
		runLog.EndTime,
		runLog.ErrorMessage); err != nil {
		r.logger.Error("Ошибка при обновлении записи в журнале ETL: %v", err)
	}
}

// StartScheduler запускает планировщик для регулярного выполнения ETL
func (r *ETLRunner) StartScheduler(ctx context.Context) {
	scheduler := gocron.NewScheduler(time.UTC)

	r.logger.Info("Запуск планировщика ETL с интервалом %v", r.config.RunInterval)

	_, err := scheduler.Every(r.config.RunInterval).Do(func() {
		r.logger.Info("Запланированный запуск ETL процесса")
		if err := r.ExecuteETL(); err != nil {
			r.logger.Error("Ошибка при выполнении запланированного ETL: %v", err)
		}
	})

	if err != nil {
		r.logger.Error("Ошибка при настройке планировщика: %v", err)
		return
	}

	// Запускаем планировщик
	scheduler.StartAsync()

	// Ожидаем сигнал остановки из контекста
	<-ctx.Done()

	// Останавливаем планировщик
	scheduler.Stop()
	r.logger.Info("Планировщик ETL остановлен")
}

// RunOnce запускает ETL процесс один раз
func RunOnce() {
	runner, err := NewETLRunner()
	if err != nil {
		log.Fatalf("Ошибка при создании ETL Runner: %v", err)
	}
	defer runner.Close()

	if err := runner.ExecuteETL(); err != nil {
		log.Fatalf("Ошибка при выполнении ETL: %v", err)
	}
}

// RunScheduled запускает ETL процесс по расписанию
func RunScheduled() {
	// Создаем контекст, который будет отменен при получении сигнала завершения
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Настраиваем обработку сигналов завершения
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

	// Запускаем горутину для обработки сигналов
	go func() {
		<-signalCh
		log.Println("Получен сигнал завершения. Останавливаем ETL Runner...")
		cancel()
	}()

	runner, err := NewETLRunner()
	if err != nil {
		log.Fatalf("Ошибка при создании ETL Runner: %v", err)
	}
	defer runner.Close()

	// Запускаем планировщик
	runner.StartScheduler(ctx)
}

// runLinearRegression запускает процесс линейной регрессии
func (r *ETLRunner) runLinearRegression() error {
	// Используем стандартную конфигурацию
	config := linear_regression.DefaultConfig()

	// Запускаем линейную регрессию с использованием OLAP базы данных
	return linear_regression.RunWithCustomConfig(r.dbConnections.OLAPDB, r.logger, config)
}

// runLinearRegressionWithParams запускает процесс линейной регрессии с пользовательскими параметрами
func (r *ETLRunner) runLinearRegressionWithParams(days, forecast int, confidence, minR2 float64) error {
	// Создаем конфигурацию с пользовательскими параметрами
	config := linear_regression.Config{
		AnalysisPeriodDays: days,
		ForecastDays:       forecast,
		ConfidenceLevel:    confidence,
		MinR2Threshold:     minR2,
	}

	r.logger.Info("Запуск линейной регрессии с параметрами: дней=%d, прогноз=%d дней, доверие=%.2f, минR²=%.2f",
		days, forecast, confidence, minR2)

	// Запускаем линейную регрессию с пользовательской конфигурацией
	return linear_regression.RunWithCustomConfig(r.dbConnections.OLAPDB, r.logger, config)
}

// RunLinearRegression запускает только линейную регрессию с пользовательскими параметрами
func RunLinearRegression(days, forecast int, confidence, minR2 float64) {
	log.Println("Запуск утилиты линейной регрессии")

	// Создаем ETL Runner
	runner, err := NewETLRunner()
	if err != nil {
		log.Fatalf("Ошибка при создании ETL Runner: %v", err)
	}
	defer runner.Close()

	// Запускаем только линейную регрессию
	if err := runner.runLinearRegressionWithParams(days, forecast, confidence, minR2); err != nil {
		log.Fatalf("Ошибка при выполнении линейной регрессии: %v", err)
	}

	log.Println("Линейная регрессия успешно завершена")
}

func main() {
	// Параметры командной строки
	modePtr := flag.String("mode", "scheduled", "Режим работы: scheduled, once или lr")
	daysPtr := flag.Int("days", 30, "Количество дней для анализа (только для режима lr)")
	forecastPtr := flag.Int("forecast", 14, "Количество дней для прогноза (только для режима lr)")
	confidencePtr := flag.Float64("confidence", 0.95, "Уровень доверия (только для режима lr)")
	minR2Ptr := flag.Float64("min-r2", 0.30, "Минимальный порог для R² (только для режима lr)")

	flag.Parse()

	log.Println("Запуск ETL Runner в режиме:", *modePtr)

	switch *modePtr {
	case "once":
		RunOnce()
	case "scheduled":
		RunScheduled()
	case "lr":
		RunLinearRegression(*daysPtr, *forecastPtr, *confidencePtr, *minR2Ptr)
	default:
		log.Println("Неизвестный режим работы:", *modePtr)
		log.Println("Доступные режимы: scheduled, once, lr")
		os.Exit(1)
	}

	log.Println("ETL Runner завершил работу")
}
