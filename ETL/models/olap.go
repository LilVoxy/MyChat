package models

import (
	"time"
)

// TimeDimension представляет временное измерение в OLAP
type TimeDimension struct {
	ID         int
	FullDate   time.Time
	Year       int
	Quarter    int
	Month      int
	MonthName  string
	WeekOfYear int
	DayOfMonth int
	DayOfWeek  int
	DayName    string
	IsWeekend  bool
	HourOfDay  int
}

// UserDimension представляет измерение пользователей в OLAP
type UserDimension struct {
	ID                     int
	RegistrationDate       time.Time
	DaysActive             int
	TotalChats             int
	TotalMessages          int
	AvgResponseTimeMinutes float64
	ActivityLevel          string // 'high', 'medium', 'low'
	LastUpdated            time.Time
}

// MessageFact представляет факт сообщения в OLAP
type MessageFact struct {
	ID                  int
	MessageID           int
	TimeID              int
	SenderID            int
	RecipientID         int
	ChatID              int
	MessageLength       int
	ResponseTimeMinutes float64
	IsFirstInChat       bool
}

// ChatFact представляет факт чата в OLAP
type ChatFact struct {
	ID                     int
	ChatID                 int
	StartTimeID            int
	EndTimeID              int
	BuyerID                int
	SellerID               int
	TotalMessages          int
	BuyerMessages          int
	SellerMessages         int
	AvgMessageLength       float64
	AvgResponseTimeMinutes float64
	ChatDurationHours      float64
}

// DailyActivityFact представляет факт ежедневной активности в OLAP
type DailyActivityFact struct {
	ID                     int
	DateID                 int
	TotalMessages          int
	TotalNewChats          int
	ActiveUsers            int
	NewUsers               int
	AvgMessagesPerChat     float64
	AvgResponseTimeMinutes float64
	PeakHour               int
	PeakHourMessages       int
}

// HourlyActivityFact представляет факт почасовой активности в OLAP
type HourlyActivityFact struct {
	ID                     int
	DateID                 int
	HourOfDay              int
	TotalMessages          int
	TotalNewChats          int
	ActiveUsers            int
	AvgResponseTimeMinutes float64
}

// ETLMetadata содержит метаданные о запуске ETL
type ETLMetadata struct {
	LastRunTimestamp       time.Time
	LastProcessedMessageID int
	LastProcessedChatID    int
	MessagesProcessed      int
	ChatsProcessed         int
	TimeDimensionUpdated   bool
	UserDimensionUpdated   bool
	ErrorsEncountered      int
}
