// database/chat.go
package database

import (
	"database/sql"
	"log"
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

// GetOrCreateChat находит существующий чат между пользователями для указанного товара
// или создает новый, если такого чата еще нет.
// Функция определяет, кто является покупателем, а кто продавцом, на основе переданных идентификаторов.
func GetOrCreateChat(user1ID, user2ID, productID int) (int, error) {
	// Определяем, кто является покупателем, а кто продавцом
	// Это должно быть определено логикой вашего приложения
	// Здесь для примера мы просто предполагаем, что первый пользователь - покупатель,
	// а второй - продавец
	buyerID := user1ID
	sellerID := user2ID

	var chatID int

	// Сначала попытаемся найти существующий чат
	err := DB.QueryRow(
		"SELECT id FROM chats WHERE buyer_id = ? AND seller_id = ? AND product_id = ?",
		buyerID, sellerID, productID,
	).Scan(&chatID)

	// Если чат не найден, пробуем поменять местами покупателя и продавца
	if err == sql.ErrNoRows {
		err = DB.QueryRow(
			"SELECT id FROM chats WHERE buyer_id = ? AND seller_id = ? AND product_id = ?",
			sellerID, buyerID, productID,
		).Scan(&chatID)

		// Если и так не нашли, значит чата действительно нет - создаем новый
		if err == sql.ErrNoRows {
			res, err := DB.Exec(
				"INSERT INTO chats (buyer_id, seller_id, product_id) VALUES (?, ?, ?)",
				buyerID, sellerID, productID,
			)
			if err != nil {
				log.Printf("Ошибка создания чата: %v", err)
				return 0, err
			}

			lastID, err := res.LastInsertId()
			if err != nil {
				log.Printf("Ошибка получения ID чата: %v", err)
				return 0, err
			}

			chatID = int(lastID)
			log.Printf("✅ Создан новый чат ID=%d между пользователями %d и %d для товара %d",
				chatID, buyerID, sellerID, productID)
		} else if err != nil {
			log.Printf("Ошибка поиска чата: %v", err)
			return 0, err
		}
	} else if err != nil {
		log.Printf("Ошибка поиска чата: %v", err)
		return 0, err
	}

	return chatID, nil
}

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

// DeleteChat удаляет чат и все его сообщения
func DeleteChat(chatID int) error {
	// Начинаем транзакцию
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	// Удаляем все сообщения чата
	_, err = tx.Exec("DELETE FROM messages WHERE chat_id = ?", chatID)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Удаляем сам чат
	_, err = tx.Exec("DELETE FROM chats WHERE id = ?", chatID)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Подтверждаем транзакцию
	return tx.Commit()
}
