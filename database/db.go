// database/db.go
package database

import (
	"database/sql"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

// InitDB инициализирует соединение с базой данных
// В этой функции больше нет прямого подключения, так как оно производится в main.go
func InitDB() {
	// Теперь функция пустая, так как подключение выполняется в main.go,
	// а переменная DB инициализируется там же
	if DB == nil {
		log.Println("⚠️ Предупреждение: переменная DB еще не инициализирована")
	}
}
