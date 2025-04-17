// database/chat.go
package database

import (
	"time"
)

// Chat представляет чат между покупателем и продавцом по конкретному товару
type Chat struct {
	ID          int
	BuyerID     int
	SellerID    int
	ProductID   int
	CreatedAt   time.Time
	LastMessage *Message
}
