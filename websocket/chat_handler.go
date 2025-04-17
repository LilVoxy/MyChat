// websocket/chat_handler.go
package websocket

import (
	"database/sql"
	"fmt"
	"log"
)

// getChatID получает ID существующего чата или создает новый
func (manager *Manager) getChatID(fromID, toID, productID int) (int, error) {
	var chatID int

	// Проверяем подключение к БД
	if manager.DB == nil {
		return 0, fmt.Errorf("❌ Отсутствует подключение к базе данных")
	}

	// Проверяем корректность входных данных
	if fromID <= 0 || toID <= 0 || productID <= 0 {
		return 0, fmt.Errorf("❌ Некорректные ID пользователей или товара: fromID=%d, toID=%d, productID=%d", fromID, toID, productID)
	}

	// Проверка существования чата
	err := manager.DB.QueryRow(`
		SELECT id FROM chats 
		WHERE (buyer_id = ? AND seller_id = ? AND product_id = ?)
		OR (buyer_id = ? AND seller_id = ? AND product_id = ?)
	`, fromID, toID, productID, toID, fromID, productID).Scan(&chatID)

	if err == nil {
		log.Printf("✅ Найден существующий чат (ID: %d)", chatID)
		return chatID, nil
	}

	if err != sql.ErrNoRows {
		log.Printf("❌ Ошибка при поиске существующего чата: %v", err)
		return 0, err
	}

	// Определяем роли (кто покупатель, кто продавец)
	// По умолчанию считаем отправителя покупателем, а получателя продавцом
	buyerID := fromID
	sellerID := toID

	// Проверяем товар, чтобы выяснить, кто продавец
	var realSellerID int
	err = manager.DB.QueryRow("SELECT seller_id FROM products WHERE id = ?", productID).Scan(&realSellerID)
	if err == nil && realSellerID > 0 {
		// Если отправитель является продавцом этого товара, меняем роли
		if realSellerID == fromID {
			buyerID = toID
			sellerID = fromID
		} else if realSellerID == toID {
			buyerID = fromID
			sellerID = toID
		}
		log.Printf("✅ Определены роли: покупатель=%d, продавец=%d (товар принадлежит продавцу %d)", buyerID, sellerID, realSellerID)
	} else if err != sql.ErrNoRows {
		log.Printf("⚠️ Не удалось определить продавца товара %d: %v, используем стандартные роли", productID, err)
	}

	// Создаем новый чат, если не существует
	stmt, err := manager.DB.Prepare(`
		INSERT INTO chats (buyer_id, seller_id, product_id, created_at)
		VALUES (?, ?, ?, NOW())
	`)
	if err != nil {
		log.Printf("❌ Ошибка подготовки запроса для создания чата: %v", err)
		return 0, err
	}
	defer stmt.Close()

	result, err := stmt.Exec(buyerID, sellerID, productID)
	if err != nil {
		log.Printf("❌ Ошибка выполнения запроса для создания чата: %v", err)
		return 0, err
	}

	newChatID, err := result.LastInsertId()
	if err != nil {
		log.Printf("❌ Ошибка получения ID нового чата: %v", err)
		return 0, err
	}

	log.Printf("✅ Создан новый чат (ID: %d, Покупатель: %d, Продавец: %d, Товар: %d)", newChatID, buyerID, sellerID, productID)
	return int(newChatID), nil
}
