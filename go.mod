module github.com/LilVoxy/coursework_chat

go 1.24.0

replace github.com/LilVoxy/coursework_chat => .

require (
	github.com/go-sql-driver/mysql v1.9.1
	github.com/golang/snappy v1.0.0
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.3
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
)
