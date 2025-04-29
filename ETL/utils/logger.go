package utils

import (
	"fmt"
	"log"
	"os"
	"time"
)

// ETLLogger представляет логгер для ETL-процесса
type ETLLogger struct {
	infoLogger  *log.Logger
	errorLogger *log.Logger
	debugLogger *log.Logger
	isVerbose   bool
}

// NewETLLogger создает новый экземпляр логгера для ETL
func NewETLLogger(verbose bool) *ETLLogger {
	// Создаем или открываем лог-файл для записи
	currentTime := time.Now().Format("2006-01-02")
	logFileName := fmt.Sprintf("etl_log_%s.log", currentTime)

	file, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Не удалось открыть или создать файл лога: %v", err)
	}

	// Инициализируем логгеры для разных уровней
	infoLogger := log.New(file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	errorLogger := log.New(file, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	debugLogger := log.New(file, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)

	return &ETLLogger{
		infoLogger:  infoLogger,
		errorLogger: errorLogger,
		debugLogger: debugLogger,
		isVerbose:   verbose,
	}
}

// Info логирует информационное сообщение
func (l *ETLLogger) Info(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	l.infoLogger.Println(msg)

	// Также выводим в стандартный вывод
	log.Println("INFO:", msg)
}

// Error логирует сообщение об ошибке
func (l *ETLLogger) Error(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	l.errorLogger.Println(msg)

	// Также выводим в стандартный вывод
	log.Println("ERROR:", msg)
}

// Debug логирует отладочное сообщение (только если включен verbose режим)
func (l *ETLLogger) Debug(format string, v ...interface{}) {
	if !l.isVerbose {
		return
	}

	msg := fmt.Sprintf(format, v...)
	l.debugLogger.Println(msg)

	// Также выводим в стандартный вывод
	log.Println("DEBUG:", msg)
}

// LogETLStart логирует начало ETL-процесса
func (l *ETLLogger) LogETLStart() {
	l.Info("Начало выполнения ETL-процесса")
}

// LogETLComplete логирует завершение ETL-процесса
func (l *ETLLogger) LogETLComplete(startTime time.Time, totalMessages int, totalUsers int, totalChats int) {
	duration := time.Since(startTime)
	l.Info("ETL-процесс завершён. Длительность: %v", duration)
	l.Info("Обработано: %d сообщений, %d пользователей, %d чатов", totalMessages, totalUsers, totalChats)
}

// LogExtractStart логирует начало фазы извлечения данных
func (l *ETLLogger) LogExtractStart() {
	l.Info("Начало фазы Extract (Извлечение данных)")
}

// LogExtractComplete логирует завершение фазы извлечения данных
func (l *ETLLogger) LogExtractComplete(messages int, users int, chats int, duration time.Duration) {
	l.Info("Фаза Extract завершена. Длительность: %v", duration)
	l.Info("Извлечено: %d сообщений, %d пользователей, %d чатов", messages, users, chats)
}
