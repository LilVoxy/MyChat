package chatrank

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// MySQLChatRankRepository реализация ChatRankRepository для MySQL
type MySQLChatRankRepository struct {
	db     *sql.DB
	logger *utils.ETLLogger
}

// NewMySQLChatRankRepository создает новый экземпляр MySQLChatRankRepository
func NewMySQLChatRankRepository(db *sql.DB, logger *utils.ETLLogger) *MySQLChatRankRepository {
	return &MySQLChatRankRepository{
		db:     db,
		logger: logger,
	}
}

// SaveUserRanks сохраняет ранги пользователей в БД
func (r *MySQLChatRankRepository) SaveUserRanks(ranks []UserInfluenceRank) error {
	if len(ranks) == 0 {
		return nil
	}

	// Используем транзакцию для атомарной записи
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("ошибка при создании транзакции: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Подготавливаем запрос с ON DUPLICATE KEY UPDATE для обновления по user_id
	stmt, err := tx.Prepare(`
		INSERT INTO chat_analytics.user_influence_rank 
		(user_id, chat_rank, rank_percentile, category, calculation_date, iteration_count, convergence_delta)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		chat_rank = VALUES(chat_rank),
		rank_percentile = VALUES(rank_percentile),
		category = VALUES(category),
		calculation_date = VALUES(calculation_date),
		iteration_count = VALUES(iteration_count),
		convergence_delta = VALUES(convergence_delta)
	`)
	if err != nil {
		return fmt.Errorf("ошибка при подготовке запроса: %w", err)
	}
	defer stmt.Close()

	// Вставляем ранги
	for _, rank := range ranks {
		_, err = stmt.Exec(
			rank.UserID,
			rank.ChatRank,
			rank.RankPercentile,
			rank.Category,
			rank.CalculationDate.Format("2006-01-02"),
			rank.IterationCount,
			rank.ConvergenceDelta,
		)
		if err != nil {
			return fmt.Errorf("ошибка при вставке ранга для пользователя %d: %w", rank.UserID, err)
		}
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("ошибка при фиксации транзакции: %w", err)
	}

	r.logger.Info("Сохранено %d рангов пользователей", len(ranks))
	return nil
}

// SaveCommunicationWeights сохраняет веса коммуникационных связей в БД
func (r *MySQLChatRankRepository) SaveCommunicationWeights(weights []CommunicationWeight) error {
	if len(weights) == 0 {
		return nil
	}

	// Используем транзакцию для атомарной записи
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("ошибка при создании транзакции: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Подготавливаем запрос с ON DUPLICATE KEY UPDATE для избежания ошибок дублирования
	stmt, err := tx.Prepare(`
		INSERT INTO chat_analytics.communication_weights 
		(sender_id, recipient_id, weight, time_factor, response_factor, length_factor, continuation_factor, calculation_date)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		weight = VALUES(weight),
		time_factor = VALUES(time_factor),
		response_factor = VALUES(response_factor),
		length_factor = VALUES(length_factor),
		continuation_factor = VALUES(continuation_factor)
	`)
	if err != nil {
		return fmt.Errorf("ошибка при подготовке запроса: %w", err)
	}
	defer stmt.Close()

	// Вставляем веса связей
	for _, weight := range weights {
		_, err = stmt.Exec(
			weight.SenderID,
			weight.RecipientID,
			weight.Weight,
			weight.TimeFactor,
			weight.ResponseFactor,
			weight.LengthFactor,
			weight.ContinuationFactor,
			weight.CalculationDate.Format("2006-01-02"),
		)
		if err != nil {
			return fmt.Errorf("ошибка при вставке веса связи от %d к %d: %w", weight.SenderID, weight.RecipientID, err)
		}
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("ошибка при фиксации транзакции: %w", err)
	}

	r.logger.Info("Сохранено %d весов коммуникационных связей", len(weights))
	return nil
}

// GetUserRankHistory получает историю рангов пользователя
func (r *MySQLChatRankRepository) GetUserRankHistory(userID int, startDate, endDate time.Time) ([]UserInfluenceRank, error) {
	rows, err := r.db.Query(`
		SELECT user_id, chat_rank, rank_percentile, category, calculation_date, iteration_count, convergence_delta
		FROM chat_analytics.user_influence_rank
		WHERE user_id = ? AND calculation_date BETWEEN ? AND ?
		ORDER BY calculation_date ASC
	`, userID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("ошибка при запросе истории рангов: %w", err)
	}
	defer rows.Close()

	var ranks []UserInfluenceRank
	for rows.Next() {
		var rank UserInfluenceRank
		var dateStr string
		err := rows.Scan(
			&rank.UserID,
			&rank.ChatRank,
			&rank.RankPercentile,
			&rank.Category,
			&dateStr,
			&rank.IterationCount,
			&rank.ConvergenceDelta,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка при сканировании результатов: %w", err)
		}

		// Преобразуем строку даты в time.Time
		rank.CalculationDate, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			return nil, fmt.Errorf("ошибка при парсинге даты: %w", err)
		}

		ranks = append(ranks, rank)
	}

	return ranks, nil
}

// GetTopUsersByRank получает топ-N пользователей по рангу
func (r *MySQLChatRankRepository) GetTopUsersByRank(limit int, date time.Time) ([]UserInfluenceRank, error) {
	rows, err := r.db.Query(`
		SELECT user_id, chat_rank, rank_percentile, category, calculation_date, iteration_count, convergence_delta
		FROM chat_analytics.user_influence_rank
		WHERE calculation_date = ?
		ORDER BY chat_rank DESC
		LIMIT ?
	`, date.Format("2006-01-02"), limit)
	if err != nil {
		return nil, fmt.Errorf("ошибка при запросе топ пользователей: %w", err)
	}
	defer rows.Close()

	var ranks []UserInfluenceRank
	for rows.Next() {
		var rank UserInfluenceRank
		var dateStr string
		err := rows.Scan(
			&rank.UserID,
			&rank.ChatRank,
			&rank.RankPercentile,
			&rank.Category,
			&dateStr,
			&rank.IterationCount,
			&rank.ConvergenceDelta,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка при сканировании результатов: %w", err)
		}

		// Преобразуем строку даты в time.Time
		rank.CalculationDate, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			return nil, fmt.Errorf("ошибка при парсинге даты: %w", err)
		}

		ranks = append(ranks, rank)
	}

	return ranks, nil
}

// GetCommunicationWeights получает веса коммуникационных связей
func (r *MySQLChatRankRepository) GetCommunicationWeights(date time.Time) ([]CommunicationWeight, error) {
	rows, err := r.db.Query(`
		SELECT sender_id, recipient_id, weight, time_factor, response_factor, length_factor, continuation_factor, calculation_date
		FROM chat_analytics.communication_weights
		WHERE calculation_date = ?
	`, date.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("ошибка при запросе весов связей: %w", err)
	}
	defer rows.Close()

	var weights []CommunicationWeight
	for rows.Next() {
		var weight CommunicationWeight
		var dateStr string
		err := rows.Scan(
			&weight.SenderID,
			&weight.RecipientID,
			&weight.Weight,
			&weight.TimeFactor,
			&weight.ResponseFactor,
			&weight.LengthFactor,
			&weight.ContinuationFactor,
			&dateStr,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка при сканировании результатов: %w", err)
		}

		// Преобразуем строку даты в time.Time
		weight.CalculationDate, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			return nil, fmt.Errorf("ошибка при парсинге даты: %w", err)
		}

		weights = append(weights, weight)
	}

	return weights, nil
}
