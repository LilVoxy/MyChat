package chatrank

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// RoundToThousandth округляет число до тысячных (3 знака после запятой)
func RoundToThousandth(value float64) float64 {
	return math.Round(value*1000) / 1000
}

// CalculateChatRank вычисляет ранги пользователей на основе построенного графа коммуникаций
func CalculateChatRank(
	userGraph map[int]*UserNode,
	config ChatRankConfig,
	logger *utils.ETLLogger) (ChatRankResult, error) {

	if len(userGraph) == 0 {
		return ChatRankResult{}, fmt.Errorf("пустой граф коммуникаций")
	}

	logger.Info("Начинаем расчет ChatRank для %d пользователей", len(userGraph))

	// Инициализируем начальные ранги всех пользователей равными значениями
	initialRank := 1.0 / float64(len(userGraph))
	for _, node := range userGraph {
		node.ChatRank = initialRank
		node.PrevRank = 0.0 // Нужно для первой итерации и проверки сходимости
	}

	// Запоминаем ID пользователей для стабильных итераций
	userIDs := make([]int, 0, len(userGraph))
	for userID := range userGraph {
		userIDs = append(userIDs, userID)
	}
	sort.Ints(userIDs) // Сортируем для стабильности вычислений

	// Итеративно вычисляем ChatRank
	var maxDelta float64
	var iteration int
	for iteration = 0; iteration < config.MaxIterations; iteration++ {
		// Сохраняем текущие ранги перед обновлением
		for _, userID := range userIDs {
			userGraph[userID].PrevRank = userGraph[userID].ChatRank
		}

		// Вычисляем новые ранги для каждого пользователя
		for _, userID := range userIDs {
			node := userGraph[userID]

			sum := 0.0
			for senderID, weight := range node.IncomingLinks {
				senderNode, exists := userGraph[senderID]
				if !exists {
					continue // Пропускаем отправителя, если его нет в графе
				}

				// Применяем формулу ChatRank
				contribution := senderNode.PrevRank * weight
				if senderNode.OutDegree > 0 {
					contribution /= senderNode.OutDegree
				}
				sum += contribution
			}

			// Новый ранг по формуле ChatRank
			node.ChatRank = (1 - config.DampingFactor) + config.DampingFactor*sum
		}

		// Проверяем сходимость (максимальное изменение ранга)
		maxDelta = 0.0
		for _, userID := range userIDs {
			node := userGraph[userID]
			delta := math.Abs(node.ChatRank - node.PrevRank)
			if delta > maxDelta {
				maxDelta = delta
			}
		}

		logger.Debug("Итерация %d, максимальная дельта: %.6f", iteration+1, maxDelta)

		// Если достигли желаемой точности, прерываем итерации
		if maxDelta < config.ConvergenceEpsilon {
			logger.Info("Сходимость достигнута на итерации %d, дельта: %.6f", iteration+1, maxDelta)
			break
		}
	}

	// Нормализуем ранги после завершения итераций
	userRanks := normalizeAndCategorizeRanks(userGraph, userIDs)

	// Заполняем результат
	result := ChatRankResult{
		UserRanks:        userRanks,
		IterationCount:   iteration + 1,
		ConvergenceDelta: RoundToThousandth(maxDelta),
		CalculationDate:  time.Now(),
	}

	logger.Info("Расчет ChatRank завершен за %d итераций", result.IterationCount)
	return result, nil
}

// normalizeAndCategorizeRanks нормализует ранги и категоризирует пользователей
func normalizeAndCategorizeRanks(userGraph map[int]*UserNode, userIDs []int) []UserInfluenceRank {
	// Создаем список для сортировки рангов
	ranks := make([]float64, 0, len(userGraph))
	for _, userID := range userIDs {
		ranks = append(ranks, userGraph[userID].ChatRank)
	}
	sort.Float64s(ranks)

	// Создаем результирующий список рангов
	userRanks := make([]UserInfluenceRank, 0, len(userGraph))

	// Время расчета
	calculationTime := time.Now()

	for _, userID := range userIDs {
		// Получаем percentile для ранга
		rank := userGraph[userID].ChatRank
		percentile := getPercentile(ranks, rank)

		// Определяем категорию
		var category string
		if percentile >= 0.9 {
			category = "high"
		} else if percentile >= 0.5 {
			category = "medium"
		} else {
			category = "low"
		}

		// Округляем значения для сохранения
		userRanks = append(userRanks, UserInfluenceRank{
			UserID:           userID,
			ChatRank:         RoundToThousandth(rank),
			RankPercentile:   RoundToThousandth(percentile),
			Category:         category,
			CalculationDate:  calculationTime,
			IterationCount:   0, // Будет установлено позже
			ConvergenceDelta: 0, // Будет установлено позже
		})
	}

	return userRanks
}

// getPercentile возвращает процентиль значения в отсортированном списке
func getPercentile(sortedValues []float64, value float64) float64 {
	if len(sortedValues) == 0 {
		return 0
	}

	// Находим позицию значения в отсортированном списке
	position := 0
	for i, v := range sortedValues {
		if v <= value {
			position = i
		} else {
			break
		}
	}

	// Вычисляем процентиль (от 0 до 1)
	return float64(position) / float64(len(sortedValues)-1)
}

// CalculateCommunicationWeights вычисляет веса коммуникационных связей
func CalculateCommunicationWeights(
	messagesMap map[int]map[int][]MessageInfo,
	config ChatRankConfig,
	logger *utils.ETLLogger) ([]CommunicationWeight, error) {

	logger.Info("Вычисление весов коммуникационных связей")

	// Результирующий список весов связей
	weights := make([]CommunicationWeight, 0)

	// Собираем метрики для каждой пары пользователей
	for senderID, recipients := range messagesMap {
		for recipientID, messages := range recipients {
			// Рассчитываем факторы
			timeFactor := calculateTimeFactor(messages)
			responseFactor := calculateResponseFactor(messages)
			lengthFactor := calculateLengthFactor(messages)
			messageCountFactor := calculateMessageCountFactor(messages)

			// Вычисляем итоговый вес как взвешенную сумму факторов
			weight := config.TimeFactor*timeFactor +
				config.ResponseFactor*responseFactor +
				config.LengthFactor*lengthFactor +
				config.ContinuationFactor*messageCountFactor

			// Добавляем вес связи в результат
			weights = append(weights, CommunicationWeight{
				SenderID:           senderID,
				RecipientID:        recipientID,
				Weight:             RoundToThousandth(weight),
				TimeFactor:         RoundToThousandth(timeFactor),
				ResponseFactor:     RoundToThousandth(responseFactor),
				LengthFactor:       RoundToThousandth(lengthFactor),
				ContinuationFactor: RoundToThousandth(messageCountFactor),
				CalculationDate:    time.Now(),
			})
		}
	}

	logger.Info("Вычислено %d весов коммуникационных связей", len(weights))
	return weights, nil
}

// MessageInfo содержит информацию о сообщении для расчета факторов
type MessageInfo struct {
	ResponseTimeMinutes float64 // Время ответа в минутах
	MessageLength       int     // Длина сообщения
	HasResponse         bool    // Имеет ли сообщение ответ
	FollowUpMessages    int     // Количество сообщений, следующих за ответом
}

// calculateTimeFactor вычисляет временной фактор (скорость ответа)
func calculateTimeFactor(messages []MessageInfo) float64 {
	if len(messages) == 0 {
		return 0
	}

	// Находим среднее время ответа
	var totalResponseTime float64
	var count int

	for _, msg := range messages {
		if msg.ResponseTimeMinutes > 0 {
			totalResponseTime += msg.ResponseTimeMinutes
			count++
		}
	}

	if count == 0 {
		return 0
	}

	avgResponseTime := totalResponseTime / float64(count)

	// Используем экспоненциальную функцию затухания
	// Чем меньше время ответа, тем выше фактор (ближе к 1)
	// Параметр 0.01 определяет скорость затухания
	return math.Exp(-0.01 * avgResponseTime)
}

// calculateResponseFactor вычисляет фактор частоты ответов
func calculateResponseFactor(messages []MessageInfo) float64 {
	if len(messages) == 0 {
		return 0
	}

	// Считаем количество сообщений с ответами
	var responsesCount int

	for _, msg := range messages {
		if msg.HasResponse {
			responsesCount++
		}
	}

	// Отношение количества ответов к общему количеству сообщений
	return float64(responsesCount) / float64(len(messages))
}

// calculateLengthFactor вычисляет фактор длины сообщений
func calculateLengthFactor(messages []MessageInfo) float64 {
	if len(messages) == 0 {
		return 0
	}

	// Вычисляем среднюю длину сообщений
	var totalLength int

	for _, msg := range messages {
		totalLength += msg.MessageLength
	}

	avgLength := float64(totalLength) / float64(len(messages))

	// Нормализуем длину относительно "стандартной" длины (например, 50 символов)
	// и ограничиваем сверху и снизу для предотвращения доминирования фактора
	standardLength := 50.0
	lengthRatio := avgLength / standardLength

	return 0.5 + 0.5*math.Min(lengthRatio, 2.0)
}

// calculateMessageCountFactor вычисляет фактор количества сообщений
func calculateMessageCountFactor(messages []MessageInfo) float64 {
	if len(messages) == 0 {
		return 0
	}

	// Используем прямое количество сообщений с нормализацией
	// для предотвращения доминирования очень активных пользователей
	messageCount := float64(len(messages))

	// Уменьшаем базовое стандартное значение с 10 до 7,
	// чтобы увеличить итоговый коэффициент
	standardCount := 7.0

	// Вместо логарифмической функции используем квадратный корень,
	// который даст более высокие значения для малых количеств сообщений
	// и добавляем множитель 1.5 для общего увеличения значений
	return 1.5 * math.Sqrt(messageCount) / math.Sqrt(standardCount)
}

// BuildUserGraph строит граф пользователей на основе весов коммуникационных связей
func BuildUserGraph(weights []CommunicationWeight) map[int]*UserNode {
	// Создаем граф пользователей
	userGraph := make(map[int]*UserNode)

	// Сначала собираем все уникальные ID пользователей
	for _, weight := range weights {
		if _, exists := userGraph[weight.SenderID]; !exists {
			userGraph[weight.SenderID] = &UserNode{
				UserID:        weight.SenderID,
				IncomingLinks: make(map[int]float64),
				OutDegree:     0,
			}
		}

		if _, exists := userGraph[weight.RecipientID]; !exists {
			userGraph[weight.RecipientID] = &UserNode{
				UserID:        weight.RecipientID,
				IncomingLinks: make(map[int]float64),
				OutDegree:     0,
			}
		}
	}

	// Заполняем связи и исходящие степени
	for _, weight := range weights {
		// Добавляем связь от отправителя к получателю
		userGraph[weight.RecipientID].IncomingLinks[weight.SenderID] = weight.Weight

		// Увеличиваем исходящую степень отправителя
		userGraph[weight.SenderID].OutDegree += weight.Weight
	}

	return userGraph
}

// RunWithCustomConfig запускает ChatRank с пользовательской конфигурацией
func RunWithCustomConfig(
	dataService DataService,
	repository ChatRankRepository,
	logger *utils.ETLLogger,
	config ChatRankConfig) error {

	startTime := time.Now()
	
	// 1. Извлекаем данные для расчета факторов
	logger.Info("Извлечение данных для ChatRank")
	messagesMap, err := dataService.GetMessagesForChatRank()
	if err != nil {
		return fmt.Errorf("ошибка при извлечении данных: %w", err)
	}

	// 2. Вычисляем веса коммуникационных связей
	logger.Info("Вычисление весов коммуникационных связей")
	weightsStartTime := time.Now()
	weights, err := CalculateCommunicationWeights(messagesMap, config, logger)
	if err != nil {
		return fmt.Errorf("ошибка при вычислении весов связей: %w", err)
	}
	logger.Info("Веса коммуникационных связей вычислены за %v", time.Since(weightsStartTime))

	// 3. Строим граф пользователей
	logger.Info("Построение графа пользователей")
	graphStartTime := time.Now()
	userGraph := BuildUserGraph(weights)
	logger.Info("Граф пользователей построен за %v", time.Since(graphStartTime))

	// 4. Запускаем алгоритм ChatRank
	logger.Info("Запуск алгоритма ChatRank")
	rankStartTime := time.Now()
	result, err := CalculateChatRank(userGraph, config, logger)
	if err != nil {
		return fmt.Errorf("ошибка при вычислении ChatRank: %w", err)
	}
	logger.Info("Алгоритм ChatRank выполнен за %v", time.Since(rankStartTime))

	// 5. Сохраняем результаты
	logger.Info("Сохранение результатов")
	saveStartTime := time.Now()
	
	// Устанавливаем недостающие поля для всех рангов
	for i := range result.UserRanks {
		result.UserRanks[i].IterationCount = result.IterationCount
		result.UserRanks[i].ConvergenceDelta = result.ConvergenceDelta
	}

	// Сохраняем ранги пользователей
	if err := repository.SaveUserRanks(result.UserRanks); err != nil {
		return fmt.Errorf("ошибка при сохранении рангов пользователей: %w", err)
	}

	// Сохраняем веса коммуникационных связей
	if err := repository.SaveCommunicationWeights(weights); err != nil {
		return fmt.Errorf("ошибка при сохранении весов связей: %w", err)
	}
	logger.Info("Результаты сохранены за %v", time.Since(saveStartTime))

	executionTime := time.Since(startTime)
	logger.Info("ChatRank успешно выполнен. Общее время выполнения: %v", executionTime)
	return nil
}

// Run запускает ChatRank с конфигурацией по умолчанию
func Run(dataService DataService, repository ChatRankRepository, logger *utils.ETLLogger) error {
	return RunWithCustomConfig(dataService, repository, logger, DefaultConfig())
}

// DataService интерфейс для получения данных для ChatRank
type DataService interface {
	// GetMessagesForChatRank возвращает карту сообщений для расчета ChatRank
	// в формате: senderID -> recipientID -> []MessageInfo
	GetMessagesForChatRank() (map[int]map[int][]MessageInfo, error)
}
