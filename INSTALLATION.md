# Инструкция по установке и запуску сервера чата

## Содержание

- [Требования к системе](#требования-к-системе)
- [Установка](#установка)
  - [Клонирование репозитория](#1-клонирование-репозитория)
  - [Настройка базы данных](#2-настройка-базы-данных)
  - [Настройка конфигурации](#3-настройка-конфигурации)
  - [Загрузка зависимостей](#4-загрузка-зависимостей)
- [Запуск сервера](#запуск-сервера)
  - [Режим разработки](#режим-разработки)
  - [Сборка и запуск исполняемого файла](#сборка-и-запуск-исполняемого-файла)
  - [Параметры командной строки](#параметры-командной-строки)
- [Тестирование установки](#тестирование-установки)
- [Настройка для производственного использования](#настройка-для-производственного-использования)
  - [Настройка HTTPS](#1-настройка-https)
  - [Настройка Supervisor](#2-настройка-supervisor-для-управления-процессом)
  - [Настройка Nginx](#3-настройка-nginx-в-качестве-обратного-прокси-опционально)
- [Мониторинг и журналирование](#мониторинг-и-журналирование)
- [Устранение неполадок](#устранение-неполадок)
- [Производительность](#производительность)
- [Обновление системы](#обновление-системы)

## Требования к системе

- **Go** версии 1.24.0 или выше
- **MySQL** 8.0 или выше
- **Git** для клонирования репозитория
- Минимальные требования к ресурсам:
  - 1 ГБ ОЗУ (рекомендуется 2+ ГБ)
  - 2 ядра процессора
  - 1 ГБ свободного дискового пространства
- Поддерживаемые ОС:
  - Linux (Ubuntu 20.04+, CentOS 8+)
  - Windows 10/11, Windows Server 2019+
  - macOS 11+

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

Создайте файл `.env` в корневой директории проекта со следующим содержимым:

```
DB_USERNAME=chatuser
DB_PASSWORD=your_password
DB_HOST=localhost
DB_PORT=3306
DB_NAME=chatdb
SERVER_PORT=8080
ENABLE_COMPRESSION=true
ENABLE_ENCRYPTION=true
```

Альтернативно можно отредактировать настройки напрямую в файле `websocket/server.go`:

```go
// Настройки для подключения к базе данных
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

### Параметры командной строки

Сервер поддерживает следующие параметры командной строки:

- `--port=8080` - порт для запуска сервера (по умолчанию 8080)
- `--config=config.json` - путь к файлу конфигурации (опционально)
- `--debug` - включение подробного логирования

Пример:
```bash
./chat_server --port=9000 --debug
```

По умолчанию сервер запускается на порту 8080. Вы можете проверить его работу, открыв в браузере:

```
http://localhost:8080
```

## Тестирование установки

Чтобы убедиться, что все компоненты работают правильно, выполните следующие проверки:

1. **Тестирование HTTP API**:
   ```bash
   curl "http://localhost:8080/api/health"
   ```
   Ожидаемый ответ: `{"status":"ok"}`

2. **Тестирование WebSocket соединения**:
   Используйте примеры кода из раздела [Тестирование WebSocket соединения](TEST_EXAMPLES.md#тестирование-websocket-соединения).

## Настройка для производственного использования

### 1. Настройка HTTPS

Для производственной среды рекомендуется использовать HTTPS. Вы можете использовать сертификаты Let's Encrypt или ваши собственные SSL-сертификаты.

1. Получите SSL-сертификаты и ключи
2. Измените код в main.go для поддержки SSL:

```go
// Запуск с SSL
err = http.ListenAndServeTLS(":443", "/path/to/cert.pem", "/path/to/key.pem", router)
if err != nil {
    log.Fatal("ListenAndServeTLS: ", err)
}
```

### 2. Настройка Supervisor для управления процессом

Supervisor помогает автоматически перезапускать сервис в случае сбоев.

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
environment=DB_HOST=localhost,DB_PORT=3306,DB_USERNAME=chatuser,DB_PASSWORD=your_password,DB_NAME=chatdb
```

После создания конфигурационного файла выполните:

```bash
supervisorctl reread
supervisorctl update
supervisorctl start chat_server
```

### 3. Настройка Nginx в качестве обратного прокси (опционально)

Nginx может использоваться для балансировки нагрузки, кэширования и обслуживания статических файлов.

```nginx
server {
    listen 80;
    server_name your-domain.com;
    
    # Перенаправление HTTP на HTTPS
    location / {
        return 301 https://$host$request_uri;
    }
}

server {
    listen 443 ssl;
    server_name your-domain.com;
    
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    # Настройки безопасности SSL
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers on;
    ssl_ciphers 'ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384';
    
    # Обслуживание статических файлов
    location /static/ {
        alias /path/to/coursework_chat/public/;
        expires 1d;
    }
    
    # Проксирование запросов к API
    location /api/ {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
    
    # Проксирование WebSocket соединений
    location /ws/ {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Настройки таймаутов WebSocket
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
    }
    
    # Все остальные запросы
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Мониторинг и журналирование

По умолчанию сервер ведет журнал в стандартный вывод. Для более продвинутого журналирования рекомендуется использовать один из следующих пакетов:

- [zerolog](https://github.com/rs/zerolog) - высокопроизводительное журналирование с поддержкой JSON
- [zap](https://github.com/uber-go/zap) - быстрое структурированное журналирование
- [logrus](https://github.com/sirupsen/logrus) - структурированное журналирование с поддержкой хуков

Для мониторинга работы сервера рекомендуется использовать:

- [Prometheus](https://prometheus.io/) + [Grafana](https://grafana.com/) для сбора метрик и визуализации
- [Sentry](https://sentry.io/) для отслеживания ошибок
- [ELK Stack](https://www.elastic.co/elastic-stack) для централизованного сбора и анализа логов

## Устранение неполадок

### Проблемы с подключением к базе данных

1. Проверьте настройки подключения к БД в конфигурационном файле или переменных окружения
2. Убедитесь, что MySQL сервер запущен:
   ```bash
   # Linux
   sudo systemctl status mysql
   
   # Windows
   sc query mysql
   ```
3. Проверьте доступность базы данных:
   ```bash
   mysql -u chatuser -p -e "SHOW DATABASES;"
   ```
4. Убедитесь, что схема базы данных корректно импортирована:
   ```bash
   mysql -u chatuser -p -e "USE chatdb; SHOW TABLES;"
   ```

### Проблемы с WebSocket соединением

1. Проверьте, что порт не блокируется брандмауэром:
   ```bash
   # Linux
   sudo ufw allow 8080/tcp
   
   # Windows
   netsh advfirewall firewall add rule name="Chat Server" dir=in action=allow protocol=TCP localport=8080
   ```
2. Проверьте логи сервера на наличие ошибок
3. Используйте инструменты отладки браузера для анализа WebSocket соединений (DevTools > Network > WS)
4. Если используется Nginx, проверьте настройки проксирования WebSocket

### Общие проблемы

1. **Ошибка "Too many open files"**:
   Увеличьте лимит открытых файлов в системе:
   ```bash
   # Временно
   ulimit -n 65536
   
   # Постоянно (добавьте в /etc/security/limits.conf)
   # www-data soft nofile 65536
   # www-data hard nofile 65536
   ```

2. **Высокая нагрузка на CPU**:
   - Проверьте настройки соединений с базой данных
   - Уменьшите частоту отправки ping-сообщений
   - Рассмотрите возможность горизонтального масштабирования

## Производительность

Сервер оптимизирован для параллельной обработки соединений. Для повышения производительности:

1. Увеличьте количество соединений с базой данных:
   ```go
   db.SetMaxOpenConns(100)
   db.SetMaxIdleConns(25)
   ```
2. Настройте таймауты для соединений:
   ```go
   db.SetConnMaxLifetime(10 * time.Minute)
   ```
3. Включите сжатие для WebSocket соединений (уменьшает трафик на 60-80%)
4. Используйте кэширование часто запрашиваемых данных
5. Мониторьте систему и выявляйте узкие места производительности

## Обновление системы

Для обновления сервера до последней версии:

1. Остановите текущий сервер
   ```bash
   supervisorctl stop chat_server
   ```
   
2. Получите последние изменения из репозитория
   ```bash
   cd /path/to/coursework_chat
   git pull origin main
   ```
   
3. Соберите новую версию
   ```bash
   go build -o chat_server
   ```
   
4. Перезапустите сервер
   ```bash
   supervisorctl start chat_server
   ```
   
При обновлении базы данных всегда делайте резервную копию перед применением миграций:
```bash
mysqldump -u chatuser -p chatdb > chatdb_backup_$(date +%Y%m%d).sql
``` 