// routes/api_routes.go
package routes

import (
	"database/sql"
	"net/http"

	"github.com/LilVoxy/coursework_chat/middleware"
	"github.com/LilVoxy/coursework_chat/websocket"
	"github.com/gorilla/mux"
)

// SetupRoutes настраивает все маршруты API и WebSocket
func SetupRoutes(router *mux.Router, db *sql.DB, wsManager *websocket.Manager) {
	// Применяем CORS middleware
	router.Use(middleware.CORSMiddleware)

	// WebSocket соединения
	router.HandleFunc("/ws/{userId}", wsManager.HandleConnections)

	// API статусов
	router.HandleFunc("/api/status", wsManager.HandleStatus).Methods("POST", "OPTIONS")

	// API чатов
	router.HandleFunc("/api/chats", GetChatsHandler(db)).Methods("GET", "OPTIONS")

	// API сообщений
	router.HandleFunc("/api/messages", GetMessagesHandler(db)).Methods("GET", "OPTIONS")

	// Статические файлы
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("public")))
}
