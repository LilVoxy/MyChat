module github.com/LilVoxy/coursework_chat

go 1.24.0

replace github.com/LilVoxy/coursework_chat => .

require (
	github.com/go-sql-driver/mysql v1.9.2
	github.com/golang/snappy v1.0.0
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.3
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/go-co-op/gocron v1.37.0 // indirect
	github.com/google/uuid v1.4.0 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	go.uber.org/atomic v1.9.0 // indirect
)
