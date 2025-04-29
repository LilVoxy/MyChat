 package models

import (
	"time"
)

// UserOLTP представляет пользователя в исходной OLTP базе данных
type UserOLTP struct {
	ID        int
	CreatedAt time.Time
}

// ChatOLTP представляет чат в исходной OLTP базе данных
type ChatOLTP struct {
	ID        int
	BuyerID   int
	SellerID  int
	CreatedAt time.Time
}

// MessageOLTP представляет сообщение в исходной OLTP базе данных
type MessageOLTP struct {
	ID         int
	ChatID     int
	SenderID   int
	Message    string
	CreatedAt  time.Time
	ReadStatus bool
}

// ExtractedData содержит данные, извлечённые из OLTP
type ExtractedData struct {
	Users     []UserOLTP
	Chats     []ChatOLTP
	Messages  []MessageOLTP
	LastRunTS time.Time
}
