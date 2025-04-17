// database/chat_query.go
package database

import (
	"database/sql"
)

// GetUserChats возвращает все чаты, в которых участвует пользователь
func GetUserChats(userID int) ([]Chat, error) {
	rows, err := DB.Query(`
		SELECT id, buyer_id, seller_id, product_id, created_at 
		FROM chats 
		WHERE buyer_id = ? OR seller_id = ? 
		ORDER BY created_at DESC
	`, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []Chat
	for rows.Next() {
		var chat Chat
		if err := rows.Scan(&chat.ID, &chat.BuyerID, &chat.SellerID, &chat.ProductID, &chat.CreatedAt); err != nil {
			return nil, err
		}

		// Получаем последнее сообщение для этого чата
		lastMsg, err := GetChatLastMessage(chat.ID)
		if err != nil {
			return nil, err
		}
		chat.LastMessage = lastMsg

		chats = append(chats, chat)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return chats, nil
}

// GetChatByID возвращает чат по его ID
func GetChatByID(chatID int) (*Chat, error) {
	var chat Chat
	err := DB.QueryRow(`
		SELECT id, buyer_id, seller_id, product_id, created_at 
		FROM chats 
		WHERE id = ?
	`, chatID).Scan(&chat.ID, &chat.BuyerID, &chat.SellerID, &chat.ProductID, &chat.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	// Получаем последнее сообщение для этого чата
	lastMsg, err := GetChatLastMessage(chat.ID)
	if err != nil {
		return nil, err
	}
	chat.LastMessage = lastMsg

	return &chat, nil
}
