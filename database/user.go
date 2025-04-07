// database/user.go
package database

import (
	"fmt"
	"log"
)

// EnsureUserExists проверяет, существует ли пользователь с данным ID,
// и если нет, создает запись с дефолтными значениями.
func EnsureUserExists(userID int) error {
	var exists bool
	err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)", userID).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		_, err := DB.Exec("INSERT INTO users (id, username, email) VALUES (?, ?, ?)",
			userID,
			fmt.Sprintf("user%d", userID),
			fmt.Sprintf("user%d@example.com", userID))
		if err != nil {
			return err
		}
		log.Printf("✅ User %d created in database", userID)
	}

	return nil
}
