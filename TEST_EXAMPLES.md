# Примеры тестирования чат-сервера

В этом документе представлены примеры тестирования основных функций сервера чатов с использованием различных инструментов.

## Тестирование WebSocket соединения

### Использование JavaScript в браузере

Вы можете использовать следующий JavaScript-код в консоли браузера для тестирования WebSocket-соединения:

```javascript
// Подключение к WebSocket
const userId = 123; // ID тестового пользователя
const ws = new WebSocket(`ws://localhost:8080/ws/${userId}`);

// Обработчики событий WebSocket
ws.onopen = function() {
  console.log('Соединение установлено');
};

ws.onmessage = function(event) {
  const message = JSON.parse(event.data);
  console.log('Получено сообщение:', message);
};

ws.onerror = function(error) {
  console.error('Ошибка WebSocket:', error);
};

ws.onclose = function() {
  console.log('Соединение закрыто');
};

// Функция для отправки сообщения
function sendMessage(toId, productId, content) {
  const message = {
    type: 'message',
    fromId: userId,
    toId: toId,
    productId: productId,
    content: content
  };
  ws.send(JSON.stringify(message));
  console.log('Отправлено сообщение:', message);
}

// Функция для отправки статуса
function sendStatus(status, isActive) {
  const statusMessage = {
    type: 'status',
    userId: userId,
    status: status,
    isActive: isActive
  };
  ws.send(JSON.stringify(statusMessage));
  console.log('Отправлен статус:', statusMessage);
}

// Пример использования
// sendMessage(456, 789, 'Привет! Интересует ваш товар');
// sendStatus('online', true);
```

### Использование WebSocket King

1. Откройте [WebSocket King](https://websocketking.com/)
2. В поле URL введите `ws://localhost:8080/ws/123`
3. Нажмите "Connect"
4. После успешного подключения введите следующий JSON в поле сообщения:

```json
{
  "type": "message",
  "fromId": 123,
  "toId": 456,
  "productId": 789,
  "content": "Тестовое сообщение из WebSocket King"
}
```

5. Нажмите "Send"

## Тестирование HTTP API

### Получение списка чатов

#### Использование curl

```bash
curl "http://localhost:8080/api/chats?userId=123"
```

#### Использование Postman

1. Создайте новый GET-запрос к URL `http://localhost:8080/api/chats`
2. Добавьте параметры запроса:
   - Key: `userId`, Value: `123`
3. Нажмите "Send"

### Получение истории сообщений

#### Использование curl

```bash
curl "http://localhost:8080/api/messages?userId=123&chatWith=456"
```

#### Использование Postman

1. Создайте новый GET-запрос к URL `http://localhost:8080/api/messages`
2. Добавьте параметры запроса:
   - Key: `userId`, Value: `123`
   - Key: `chatWith`, Value: `456`
3. Нажмите "Send"

### Обновление статуса пользователя

#### Использование curl

```bash
curl -X POST "http://localhost:8080/api/status" \
  -H "Content-Type: application/json" \
  -d '{"userId": 123, "status": "online", "isActive": true}'
```

#### Использование Postman

1. Создайте новый POST-запрос к URL `http://localhost:8080/api/status`
2. Выберите "Body" > "raw" > "JSON"
3. Введите следующий JSON:
   ```json
   {
     "userId": 123,
     "status": "online",
     "isActive": true
   }
   ```
4. Нажмите "Send"

## Тестирование одновременного подключения нескольких пользователей

Для тестирования взаимодействия между несколькими пользователями:

1. Откройте две (или более) вкладки в браузере
2. В каждой вкладке используйте JavaScript-код из первого примера, 
   но с разными значениями `userId` (например, 123 и 456)
3. Используйте функцию `sendMessage()` для отправки сообщений между пользователями
4. Проверьте, что сообщения появляются в консоли вкладки соответствующего получателя

## Тестирование обработки ошибок

### Тестирование неправильного формата сообщения

```javascript
// Отправка сообщения с неправильным форматом
ws.send(JSON.stringify({
  type: 'unknown_type',
  data: 'some data'
}));
```

### Тестирование таймаута соединения

1. Установите соединение WebSocket
2. Не отправляйте сообщения в течение 65 секунд
3. Проверьте, что соединение закрылось из-за таймаута

## Тестирование производительности

Для базового тестирования производительности вы можете использовать следующий скрипт, который создает несколько соединений и отправляет сообщения:

```javascript
// Количество тестовых соединений
const connectionCount = 10;
const connections = [];

// Функция создания соединения
function createConnection(userId) {
  const ws = new WebSocket(`ws://localhost:8080/ws/${userId}`);
  
  ws.onopen = function() {
    console.log(`Соединение #${userId} установлено`);
  };
  
  ws.onmessage = function(event) {
    // только регистрируем получение сообщения, не выводим для экономии ресурсов консоли
    console.log(`Соединение #${userId} получило сообщение`);
  };
  
  ws.onerror = function(error) {
    console.error(`Ошибка в соединении #${userId}`);
  };
  
  ws.onclose = function() {
    console.log(`Соединение #${userId} закрыто`);
  };
  
  return ws;
}

// Создание соединений
for (let i = 1; i <= connectionCount; i++) {
  connections.push({
    userId: i,
    ws: createConnection(i)
  });
}

// Функция отправки тестовых сообщений
function sendTestMessages() {
  connections.forEach((conn, index) => {
    if (conn.ws.readyState === WebSocket.OPEN) {
      // Отправляем сообщение следующему пользователю в списке (по кругу)
      const nextUserIndex = (index + 1) % connections.length;
      const message = {
        type: 'message',
        fromId: conn.userId,
        toId: connections[nextUserIndex].userId,
        productId: 789,
        content: `Тестовое сообщение от пользователя ${conn.userId}`
      };
      conn.ws.send(JSON.stringify(message));
    }
  });
}

// Отправка сообщений каждую секунду в течение 10 секунд
let counter = 0;
const interval = setInterval(() => {
  sendTestMessages();
  counter++;
  
  if (counter >= 10) {
    clearInterval(interval);
    console.log('Тест завершен');
  }
}, 1000);
```

## Тестирование сценариев использования

### Сценарий 1: Создание нового чата о товаре

1. Пользователь 123 (покупатель) отправляет первое сообщение пользователю 456 (продавцу) о товаре 789:

```javascript
sendMessage(456, 789, "Здравствуйте! Товар ещё доступен?");
```

2. Проверьте, что в базе данных создается новый чат
3. Получите список чатов пользователя 123:

```bash
curl "http://localhost:8080/api/chats?userId=123"
```

### Сценарий 2: Обмен сообщениями в существующем чате

1. Пользователь 123 отправляет сообщение:

```javascript
sendMessage(456, 789, "Интересует возможность доставки");
```

2. Пользователь 456 отвечает:

```javascript
// В консоли другой вкладки или другого клиента
sendMessage(123, 789, "Да, доставка возможна. Стоимость 300 рублей");
```

3. Получите историю сообщений:

```bash
curl "http://localhost:8080/api/messages?userId=123&chatWith=456"
``` 