package chatrank

import (
	"time"
)

// UserNode представляет узел пользователя в графе коммуникаций
type UserNode struct {
	UserID        int             // ID пользователя
	IncomingLinks map[int]float64 // ID отправителя -> вес связи
	OutDegree     float64         // Сумма весов исходящих связей
	ChatRank      float64         // Текущий расчетный ранг пользователя
	PrevRank      float64         // Предыдущий ранг для проверки сходимости
}

// CommunicationWeight представляет вес коммуникационной связи между пользователями
type CommunicationWeight struct {
	SenderID           int       // ID отправителя
	RecipientID        int       // ID получателя
	Weight             float64   // Итоговый вес связи
	TimeFactor         float64   // Временной фактор (скорость ответа)
	ResponseFactor     float64   // Фактор частоты ответов
	LengthFactor       float64   // Фактор длины сообщений
	ContinuationFactor float64   // Фактор количества сообщений (ранее - фактор продолжения беседы)
	CalculationDate    time.Time // Дата расчета
}

// UserInfluenceRank представляет ранг влиятельности пользователя
type UserInfluenceRank struct {
	UserID           int       // ID пользователя
	ChatRank         float64   // Значение ChatRank
	RankPercentile   float64   // Процентиль ранга
	Category         string    // Категория влияния (high, medium, low)
	CalculationDate  time.Time // Дата расчета
	IterationCount   int       // Количество итераций до сходимости
	ConvergenceDelta float64   // Дельта сходимости
}

// ChatRankConfig содержит параметры для алгоритма ChatRank
type ChatRankConfig struct {
	DampingFactor      float64 // Коэффициент затухания (обычно 0.85)
	MaxIterations      int     // Максимальное количество итераций
	ConvergenceEpsilon float64 // Порог сходимости
	TimeFactor         float64 // Вес временного фактора (alpha)
	ResponseFactor     float64 // Вес фактора частоты ответов (beta)
	LengthFactor       float64 // Вес фактора длины сообщений (gamma)
	ContinuationFactor float64 // Вес фактора количества сообщений (delta)
}

// ChatRankResult содержит результаты расчета ChatRank
type ChatRankResult struct {
	UserRanks        []UserInfluenceRank   // Рассчитанные ранги пользователей
	Weights          []CommunicationWeight // Рассчитанные веса связей
	IterationCount   int                   // Количество выполненных итераций
	ConvergenceDelta float64               // Итоговая дельта сходимости
	CalculationDate  time.Time             // Дата расчета
}

// DefaultConfig возвращает конфигурацию ChatRank по умолчанию
func DefaultConfig() ChatRankConfig {
	return ChatRankConfig{
		DampingFactor:      0.85,
		MaxIterations:      100,
		ConvergenceEpsilon: 0.0001,
		TimeFactor:         0.25,
		ResponseFactor:     0.1,
		LengthFactor:       0.30,
		ContinuationFactor: 0.35,
	}
}

// ChatRankRepository интерфейс для работы с хранилищем рангов
type ChatRankRepository interface {
	// SaveUserRanks сохраняет ранги пользователей в БД
	SaveUserRanks(ranks []UserInfluenceRank) error

	// SaveCommunicationWeights сохраняет веса коммуникационных связей в БД
	SaveCommunicationWeights(weights []CommunicationWeight) error

	// GetUserRankHistory получает историю рангов пользователя
	GetUserRankHistory(userID int, startDate, endDate time.Time) ([]UserInfluenceRank, error)

	// GetTopUsersByRank получает топ-N пользователей по рангу
	GetTopUsersByRank(limit int, date time.Time) ([]UserInfluenceRank, error)

	// GetCommunicationWeights получает веса коммуникационных связей
	GetCommunicationWeights(date time.Time) ([]CommunicationWeight, error)
}
