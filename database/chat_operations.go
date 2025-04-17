// database/chat_operations.go
package database

import (
	"database/sql"
	"log"
)

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
