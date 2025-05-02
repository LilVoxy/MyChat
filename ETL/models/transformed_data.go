package models

// TransformedData содержит трансформированные данные для загрузки в OLAP
type TransformedData struct {
	// Измерения
	Users []UserDimension

	// Факты
	Messages      []MessageFact
	Chats         []ChatFact
	DailyActivity []DailyActivityFact

	// Метаданные
	Metadata ETLMetadata
}
