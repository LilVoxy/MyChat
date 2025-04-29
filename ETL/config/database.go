package config

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// DBConnections содержит подключения к базам данных
type DBConnections struct {
	OLTPDB *sql.DB
	OLAPDB *sql.DB
}

// ConnectDatabases устанавливает подключения к базам данных OLTP и OLAP
func ConnectDatabases(config ETLConfig) (*DBConnections, error) {
	var connections DBConnections
	var err error

	// Подключение к OLTP базе данных (исходная)
	oltpDSN := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		config.OLTPConfig.User,
		config.OLTPConfig.Password,
		config.OLTPConfig.Host,
		config.OLTPConfig.Port,
		config.OLTPConfig.DBName,
	)

	connections.OLTPDB, err = sql.Open(config.OLTPConfig.Driver, oltpDSN)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к OLTP базе данных: %w", err)
	}

	// Настройка параметров подключения к OLTP
	connections.OLTPDB.SetMaxOpenConns(10)
	connections.OLTPDB.SetMaxIdleConns(5)
	connections.OLTPDB.SetConnMaxLifetime(5 * time.Minute)

	// Проверка подключения к OLTP
	if err := connections.OLTPDB.Ping(); err != nil {
		return nil, fmt.Errorf("не удалось установить соединение с OLTP базой данных: %w", err)
	}

	// Подключение к OLAP базе данных (целевая)
	olapDSN := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		config.OLAPConfig.User,
		config.OLAPConfig.Password,
		config.OLAPConfig.Host,
		config.OLAPConfig.Port,
		config.OLAPConfig.DBName,
	)

	connections.OLAPDB, err = sql.Open(config.OLAPConfig.Driver, olapDSN)
	if err != nil {
		// Закрываем первое подключение при ошибке
		connections.OLTPDB.Close()
		return nil, fmt.Errorf("ошибка подключения к OLAP базе данных: %w", err)
	}

	// Настройка параметров подключения к OLAP
	connections.OLAPDB.SetMaxOpenConns(10)
	connections.OLAPDB.SetMaxIdleConns(5)
	connections.OLAPDB.SetConnMaxLifetime(5 * time.Minute)

	// Проверка подключения к OLAP
	if err := connections.OLAPDB.Ping(); err != nil {
		// Закрываем оба подключения при ошибке
		connections.OLTPDB.Close()
		connections.OLAPDB.Close()
		return nil, fmt.Errorf("не удалось установить соединение с OLAP базой данных: %w", err)
	}

	log.Println("Успешное подключение к базам данных OLTP и OLAP")
	return &connections, nil
}

// CloseDatabases закрывает подключения к базам данных
func CloseDatabases(connections *DBConnections) {
	if connections.OLTPDB != nil {
		if err := connections.OLTPDB.Close(); err != nil {
			log.Printf("Ошибка при закрытии соединения с OLTP базой данных: %v", err)
		}
	}

	if connections.OLAPDB != nil {
		if err := connections.OLAPDB.Close(); err != nil {
			log.Printf("Ошибка при закрытии соединения с OLAP базой данных: %v", err)
		}
	}

	log.Println("Соединения с базами данных закрыты")
}
