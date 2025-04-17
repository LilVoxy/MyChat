// websocket/handler.go
package websocket

// Этот файл был разделен на три файла для улучшения организации кода:
// 1. connection_handler.go - содержит функцию HandleConnections
// 2. read_pump.go - содержит функцию readPump для чтения сообщений от клиента
// 3. write_pump.go - содержит функцию writePump для отправки сообщений клиенту
