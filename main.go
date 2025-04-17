// main.go
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LilVoxy/coursework_chat/routes"
	"github.com/LilVoxy/coursework_chat/websocket"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
)

func main() {
	fmt.Println("Запуск сервера...")

	// Инициализация базы данных
	db, err := websocket.InitDB()
	if err != nil {
		log.Fatalf("❌ Не удалось инициализировать базу данных: %v", err)
	}
	defer db.Close()

	// Создаем новый менеджер WebSocket с подключением к БД
	wsManager := websocket.NewManager(db)
	websocket.SetManager(wsManager)

	// Запускаем менеджер WebSocket
	go wsManager.Run()

	// Создаем маршрутизатор
	router := mux.NewRouter()

	// Настройка всех маршрутов
	routes.SetupRoutes(router, db, wsManager)

	// Настраиваем сервер
	server := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Запускаем сервер в отдельной горутине
	go func() {
		log.Printf("✅ Сервер запущен на http://localhost%s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Ошибка запуска сервера: %v", err)
		}
	}()

	// Канал для сигналов завершения
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Ожидаем сигнал завершения
	<-stop
	log.Println("⚠️ Получен сигнал завершения, закрываем соединения...")

	// Закрываем соединение с базой данных
	if err := db.Close(); err != nil {
		log.Printf("❌ Ошибка закрытия соединения с БД: %v", err)
	} else {
		log.Println("✅ Соединение с БД закрыто")
	}

	log.Println("👋 Сервер остановлен")
}
