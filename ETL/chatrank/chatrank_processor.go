package chatrank

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// ChatRankProcessor отвечает за вычисление и сохранение ChatRank
type ChatRankProcessor struct {
	oltpDB      *sql.DB
	olapDB      *sql.DB
	logger      *utils.ETLLogger
	dataService DataService
	repository  ChatRankRepository
	config      ChatRankConfig
}

// NewChatRankProcessor создает новый экземпляр ChatRankProcessor
func NewChatRankProcessor(oltpDB, olapDB *sql.DB, logger *utils.ETLLogger) *ChatRankProcessor {
	dataService := NewMySQLDataService(oltpDB, olapDB, logger)
	repository := NewMySQLChatRankRepository(olapDB, logger)

	return &ChatRankProcessor{
		oltpDB:      oltpDB,
		olapDB:      olapDB,
		logger:      logger,
		dataService: dataService,
		repository:  repository,
		config:      DefaultConfig(),
	}
}

// Process запускает процесс вычисления ChatRank
func (p *ChatRankProcessor) Process() error {
	startTime := time.Now()
	p.logger.Info("Запуск процесса ChatRank")

	// Запускаем ChatRank с конфигурацией по умолчанию
	err := Run(p.dataService, p.repository, p.logger)
	if err != nil {
		return fmt.Errorf("ошибка при выполнении ChatRank: %w", err)
	}

	p.logger.Info("Процесс ChatRank успешно завершен. Время выполнения: %v", time.Since(startTime))
	return nil
}

// ProcessWithCustomConfig запускает процесс вычисления ChatRank с пользовательской конфигурацией
func (p *ChatRankProcessor) ProcessWithCustomConfig(config ChatRankConfig) error {
	startTime := time.Now()
	p.logger.Info("Запуск процесса ChatRank с пользовательской конфигурацией")

	// Запускаем ChatRank с пользовательской конфигурацией
	err := RunWithCustomConfig(p.dataService, p.repository, p.logger, config)
	if err != nil {
		return fmt.Errorf("ошибка при выполнении ChatRank с пользовательской конфигурацией: %w", err)
	}

	p.logger.Info("Процесс ChatRank с пользовательской конфигурацией успешно завершен. Время выполнения: %v",
		time.Since(startTime))
	return nil
}

// SetConfig устанавливает конфигурацию для ChatRank
func (p *ChatRankProcessor) SetConfig(config ChatRankConfig) {
	p.config = config
}

// GetConfig возвращает текущую конфигурацию ChatRank
func (p *ChatRankProcessor) GetConfig() ChatRankConfig {
	return p.config
}

// GetTopUsers возвращает топ-N пользователей по ChatRank
func (p *ChatRankProcessor) GetTopUsers(n int) ([]UserInfluenceRank, error) {
	return p.repository.GetTopUsersByRank(n, time.Now())
}

// GetUserRankHistory возвращает историю рангов пользователя
func (p *ChatRankProcessor) GetUserRankHistory(userID int, daysBack int) ([]UserInfluenceRank, error) {
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -daysBack)
	return p.repository.GetUserRankHistory(userID, startDate, endDate)
}
