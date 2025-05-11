package config

import (
	"time"
)

// ETLConfig содержит конфигурацию для ETL-процесса
type ETLConfig struct {
	// Конфигурация для подключения к OLTP БД (исходной)
	OLTPConfig DatabaseConfig `json:"oltp_config"`

	// Конфигурация для подключения к OLAP БД (целевой)
	OLAPConfig DatabaseConfig `json:"olap_config"`

	// Интервал запуска ETL
	RunInterval time.Duration `json:"run_interval"`

	// Максимальное количество записей, обрабатываемых за один запуск
	BatchSize int `json:"batch_size"`

	// Периоды для агрегаций (в днях)
	AggregatePeriods struct {
		Daily  int `json:"daily"`
		Weekly int `json:"weekly"`
	} `json:"aggregate_periods"`

	// Пороговые значения для определения активности
	ActivityThresholds struct {
		High   int `json:"high"`   // Порог высокой активности (сообщений в день)
		Medium int `json:"medium"` // Порог средней активности (сообщений в день)
	} `json:"activity_thresholds"`

	// Включение/отключение логирования
	EnableDetailedLogging bool `json:"enable_detailed_logging"`
}

// DatabaseConfig содержит настройки подключения к базе данных
type DatabaseConfig struct {
	Driver   string `json:"driver"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
}

// Значения конфигурации по умолчанию
var (
	DefaultOLTPConfig = DatabaseConfig{
		Driver:   "mysql",
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Password: "Vjnbkmlf40782",
		DBName:   "chatdb",
	}

	DefaultOLAPConfig = DatabaseConfig{
		Driver:   "mysql",
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Password: "Vjnbkmlf40782",
		DBName:   "chat_analytics",
	}

	DefaultETLConfig = ETLConfig{
		OLTPConfig:            DefaultOLTPConfig,
		OLAPConfig:            DefaultOLAPConfig,
		RunInterval:           1 * time.Hour,
		BatchSize:             10000,
		EnableDetailedLogging: true,
	}
)

// GetConfig возвращает конфигурацию ETL
func GetConfig() ETLConfig {
	config := DefaultETLConfig

	// Настройка порогов активности
	config.ActivityThresholds.High = 20  // 20+ сообщений - высокая активность
	config.ActivityThresholds.Medium = 5 // 5-19 сообщений - средняя активность, <5 - низкая

	// Настройка периодов агрегации
	config.AggregatePeriods.Daily = 1
	config.AggregatePeriods.Weekly = 7

	return config
}
