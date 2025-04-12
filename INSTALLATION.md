# Инструкция по установке и запуску сервера чата

## Требования к системе

- Go версии 1.24.0 или выше
- MySQL 8.0 или выше
- Git

## Установка

### 1. Клонирование репозитория

```bash
git clone https://github.com/LilVoxy/coursework_chat.git
cd coursework_chat
```

### 2. Настройка базы данных

1. Создайте новую базу данных MySQL:

```sql
CREATE DATABASE chatdb CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

2. Создайте пользователя с доступом к базе данных или используйте существующего:

```sql
CREATE USER 'chatuser'@'localhost' IDENTIFIED BY 'your_password';
GRANT ALL PRIVILEGES ON chatdb.* TO 'chatuser'@'localhost';
FLUSH PRIVILEGES;
```

3. Импортируйте схему базы данных:

```bash
mysql -u chatuser -p chatdb < database/schema.sql
```

### 3. Настройка конфигурации

Откройте файл `websocket/server.go` и найдите блок с настройками базы данных (около строки 90):

```go
// Настройки для подключения к базе данных
dbInfo := &DBInfo{
    Username: "root",
    Password: "Vjnbkmlf40782",
    Host:     "localhost",
    Port:     "3306",
    Database: "chatdb",
}
```

Измените параметры подключения на созданные вами в шаге 2:

```go
dbInfo := &DBInfo{
    Username: "chatuser",
    Password: "your_password",
    Host:     "localhost",
    Port:     "3306",
    Database: "chatdb",
}
```

### 4. Загрузка зависимостей

```bash
go mod download
```

## Запуск сервера

### Режим разработки

```bash
go run main.go
```

### Сборка и запуск исполняемого файла

```bash
go build -o chat_server
./chat_server
```

По умолчанию сервер запускается на порту 8080. Вы можете проверить его работу, открыв в браузере:

```
http://localhost:8080
```

## Тестирование WebSocket соединения

Вы можете использовать онлайн-инструмент [WebSocket King](https://websocketking.com/) или [Postman](https://www.postman.com/) для тестирования WebSocket соединений.

1. Подключитесь к WebSocket URL:
```
ws://localhost:8080/ws/{userId}
```
Замените `{userId}` на ID пользователя, например `ws://localhost:8080/ws/123`

2. Отправьте тестовое сообщение:
```json
{
  "type": "message",
  "fromId": 123,
  "toId": 456,
  "productId": 789,
  "content": "Тестовое сообщение"
}
```

## Настройка для производственного использования

### 1. Настройка HTTPS

Для производственной среды рекомендуется использовать HTTPS. Измените код в main.go для поддержки SSL:

```go
// Запуск с SSL
err = http.ListenAndServeTLS(":443", "/path/to/cert.pem", "/path/to/key.pem", router)
if err != nil {
    log.Fatal("ListenAndServeTLS: ", err)
}
```

### 2. Настройка Supervisor для управления процессом

Пример конфигурации supervisor (/etc/supervisor/conf.d/chat_server.conf):

```ini
[program:chat_server]
command=/path/to/chat_server
directory=/path/to/coursework_chat
autostart=true
autorestart=true
stderr_logfile=/var/log/chat_server.err.log
stdout_logfile=/var/log/chat_server.out.log
user=www-data
```

### 3. Настройка Nginx в качестве обратного прокси (опционально)

```nginx
server {
    listen 80;
    server_name your-domain.com;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Мониторинг и журналирование

По умолчанию сервер ведет журнал в стандартный вывод. Вы можете настроить более продвинутое журналирование, использовав один из следующих пакетов:

- [zerolog](https://github.com/rs/zerolog)
- [zap](https://github.com/uber-go/zap)
- [logrus](https://github.com/sirupsen/logrus)

## Устранение неполадок

### Проблемы с подключением к базе данных

1. Проверьте настройки подключения к БД
2. Убедитесь, что MySQL сервер запущен:
   ```bash
   sudo systemctl status mysql
   ```
3. Проверьте доступность базы данных:
   ```bash
   mysql -u chatuser -p -e "SHOW DATABASES;"
   ```

### Проблемы с WebSocket соединением

1. Проверьте, что порт не блокируется брандмауэром:
   ```bash
   sudo ufw allow 8080/tcp
   ```
2. Проверьте логи сервера на наличие ошибок
3. Используйте инструменты отладки браузера для анализа WebSocket соединений

## Производительность

Сервер оптимизирован для параллельной обработки соединений. Для повышения производительности:

1. Увеличьте количество соединений с базой данных:
   ```go
   db.SetMaxOpenConns(50)
   db.SetMaxIdleConns(25)
   ```
2. Настройте таймауты для соединений:
   ```go
   db.SetConnMaxLifetime(10 * time.Minute)
   ``` 