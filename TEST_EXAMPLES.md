# Примеры тестирования чат-сервера

В этом документе представлены примеры тестирования основных функций сервера чатов с использованием различных инструментов.

## Содержание

- [Тестирование WebSocket соединения](#тестирование-websocket-соединения)
  - [Использование JavaScript в браузере](#использование-javascript-в-браузере)
  - [Использование WebSocket King](#использование-websocket-king)
- [Тестирование HTTP API](#тестирование-http-api)
  - [Получение списка чатов](#получение-списка-чатов)
  - [Получение истории сообщений](#получение-истории-сообщений)
  - [Обновление статуса пользователя](#обновление-статуса-пользователя)
- [Автоматизация тестирования](#автоматизация-тестирования)
  - [Тестирование с помощью Jest](#тестирование-с-помощью-jest)
  - [Нагрузочное тестирование](#нагрузочное-тестирование)
- [Тестирование одновременного подключения нескольких пользователей](#тестирование-одновременного-подключения-нескольких-пользователей)
- [Тестирование обработки ошибок](#тестирование-обработки-ошибок)
- [Тестирование производительности](#тестирование-производительности)
- [Тестирование сценариев использования](#тестирование-сценариев-использования)

## Автоматизация тестирования

### Тестирование с помощью Jest

Для автоматизации тестирования WebSocket соединений можно использовать библиотеку Jest и ws для Node.js:

```javascript
// websocket.test.js
const WebSocket = require('ws');
const { v4: uuidv4 } = require('uuid');

describe('WebSocket Chat Server', () => {
  let client1;
  let client2;
  const userId1 = 123;
  const userId2 = 456;
  
  beforeAll(() => {
    return new Promise((resolve) => {
      // Подключаем двух клиентов
      client1 = new WebSocket(`ws://localhost:8080/ws/${userId1}`);
      client2 = new WebSocket(`ws://localhost:8080/ws/${userId2}`);
      
      // Ждем подключения обоих клиентов
      let connectedCount = 0;
      
      function checkConnections() {
        connectedCount++;
        if (connectedCount === 2) {
          resolve();
        }
      }
      
      client1.on('open', checkConnections);
      client2.on('open', checkConnections);
    });
  });
  
  afterAll(() => {
    // Закрываем соединения
    client1.close();
    client2.close();
  });
  
  test('должен отправлять и получать сообщения', (done) => {
    // Уникальный идентификатор для этого тестового сообщения
    const testMessageId = uuidv4();
    const testMessage = {
      type: 'message',
      fromId: userId1,
      toId: userId2,
      productId: 789,
      content: `Тестовое сообщение ${testMessageId}`
    };
    
    // Настраиваем обработчик для второго клиента
    client2.on('message', (data) => {
      const message = JSON.parse(data);
      
      // Проверяем, что получено наше тестовое сообщение
      if (message.type === 'message' && message.content.includes(testMessageId)) {
        expect(message.fromId).toBe(userId1);
        expect(message.toId).toBe(userId2);
        expect(message.productId).toBe(789);
        done();
      }
    });
    
    // Отправляем сообщение от первого клиента
    client1.send(JSON.stringify(testMessage));
  });
  
  test('должен отправлять подтверждение отправителю', (done) => {
    // Уникальный идентификатор для этого тестового сообщения
    const testMessageId = uuidv4();
    const tempId = `temp_${Date.now()}`;
    const testMessage = {
      type: 'message',
      fromId: userId1,
      toId: userId2,
      productId: 789,
      content: `Тестовое сообщение ${testMessageId}`,
      tempId: tempId
    };
    
    // Настраиваем обработчик для первого клиента (отправителя)
    client1.on('message', (data) => {
      const message = JSON.parse(data);
      
      // Проверяем, что получено подтверждение с правильным tempId
      if (message.type === 'confirmation' && message.tempId === tempId) {
        expect(message.status).toBe('sent');
        expect(message.id).toBeTruthy(); // ID должен быть присвоен
        done();
      }
    });
    
    // Отправляем сообщение от первого клиента
    client1.send(JSON.stringify(testMessage));
  });
});
```

### Нагрузочное тестирование

Для проведения нагрузочного тестирования можно использовать инструмент Artillery:

1. Установите Artillery:
```bash
npm install -g artillery
```

2. Создайте файл конфигурации тестов `websocket-load-test.yml`:
```yaml
config:
  target: "ws://localhost:8080"
  phases:
    - duration: 60
      arrivalRate: 5
      rampTo: 50
      name: "Постепенное наращивание нагрузки"
  ws:
    # Время между ping сообщениями (в секундах)
    pingInterval: 30

scenarios:
  - name: "Обмен сообщениями"
    flow:
      # Устанавливаем соединение
      - connect:
          path: "/ws/{{ $randomNumber(1000, 9999) }}"
          
      # Ждем установки соединения
      - think: 2
      
      # Отправляем статус "online"
      - send:
          json:
            type: "status"
            userId: "{{ $randomNumber(1000, 9999) }}"
            status: "online"
            isActive: true
      
      # Отправляем несколько сообщений с интервалами
      - loop:
          - send:
              json:
                type: "message"
                fromId: "{{ $randomNumber(1000, 9999) }}"
                toId: "{{ $randomNumber(1000, 9999) }}"
                productId: "{{ $randomNumber(100, 999) }}"
                content: "Тестовое сообщение {{ $randomString(20) }}"
                tempId: "temp_{{ $timestamp }}"
          - think: 5
        count: 10
```

3. Запустите тест:
```bash
artillery run websocket-load-test.yml -o test-report.json
```

4. Сформируйте HTML-отчет:
```bash
artillery report test-report.json
```

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

// Настраиваем обработчик для получения ответа об ошибке
ws.onmessage = function(event) {
  const data = JSON.parse(event.data);
  if (data.type === 'error') {
    console.log('Получена ошибка от сервера:', data.content);
    // Здесь можно проверить корректность сообщения об ошибке
  }
};
```

### Тестирование ограничения размера сообщения

```javascript
// Создаем большое сообщение, превышающее лимит в 512 KB
const largeContent = 'a'.repeat(600 * 1024); // 600 KB
ws.send(JSON.stringify({
  type: 'message',
  fromId: 123,
  toId: 456,
  productId: 789,
  content: largeContent
}));

// Ожидаем ошибку о превышении размера
ws.onmessage = function(event) {
  const data = JSON.parse(event.data);
  if (data.type === 'error' && data.code === 'MESSAGE_TOO_LARGE') {
    console.log('Успешно получена ошибка о превышении размера');
  }
};
```

### Тестирование отключения соединения

```javascript
// Функция для тестирования переподключения
function testReconnection() {
  let reconnected = false;
  const ws = new WebSocket(`ws://localhost:8080/ws/123`);
  
  ws.onopen = function() {
    console.log('Соединение установлено');
    
    // Закрываем соединение на стороне сервера, отправляя специальное сообщение
    ws.send(JSON.stringify({
      type: 'test_command',
      command: 'close_connection'
    }));
  };
  
  ws.onclose = function() {
    console.log('Соединение закрыто, пробуем переподключиться');
    
    // Пытаемся переподключиться
    setTimeout(() => {
      const newWs = new WebSocket(`ws://localhost:8080/ws/123`);
      
      newWs.onopen = function() {
        console.log('Успешно переподключились');
        reconnected = true;
        newWs.close();
      };
    }, 1000);
  };
  
  // Проверяем результат через 5 секунд
  setTimeout(() => {
    console.log('Тест переподключения ' + 
                (reconnected ? 'успешно пройден' : 'не пройден'));
  }, 5000);
}
```

### Таймаут бездействия

1. Установите соединение WebSocket
2. Не отправляйте сообщения в течение 65 секунд
3. Проверьте, что соединение закрылось из-за таймаута

```javascript
function testIdleTimeout() {
  const ws = new WebSocket(`ws://localhost:8080/ws/123`);
  let timeoutOccurred = false;
  
  ws.onopen = function() {
    console.log('Соединение установлено, ожидаем таймаут...');
  };
  
  ws.onclose = function(event) {
    console.log('Соединение закрыто с кодом:', event.code);
    if (event.code === 1001) {
      console.log('Таймаут бездействия сработал корректно');
      timeoutOccurred = true;
    }
  };
  
  // Проверяем результат через 70 секунд (больше таймаута в 65 секунд)
  setTimeout(() => {
    console.log('Тест таймаута ' + 
                (timeoutOccurred ? 'успешно пройден' : 'не пройден'));
  }, 70000);
}
```

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

## Лучшие практики тестирования

### Комплексное тестирование

Эффективное тестирование WebSocket-приложения включает несколько уровней:

1. **Модульное тестирование** - проверка отдельных компонентов (обработчики сообщений, парсеры и т.д.)
2. **Интеграционное тестирование** - проверка взаимодействия компонентов
3. **Системное тестирование** - проверка всей системы в целом
4. **Нагрузочное тестирование** - проверка производительности под нагрузкой

### Организация тестов

Рекомендуется организовать тесты по следующим группам:

1. **Функциональные тесты** - проверка основной функциональности
   - Подключение к WebSocket
   - Отправка/получение сообщений
   - Обновление статусов
   - Работа с историей

2. **Тесты безопасности**
   - Аутентификация и авторизация
   - Защита от инъекций
   - Шифрование сообщений

3. **Тесты надежности**
   - Восстановление после сбоев
   - Обработка разрывов соединения
   - Проверка долговременной стабильности

### Инструменты для тестирования

Кроме упомянутых в документе инструментов (WebSocket King, Postman, Jest, Artillery), также полезны:

- **WebSockets.org Echo Test** - простой онлайн-тест для проверки соединения
- **Chrome DevTools** - инструменты разработчика в браузере для отладки сетевых запросов
- **Wireshark** - анализатор сетевого трафика для детального изучения WebSocket коммуникации
- **Mocha + Chai** - альтернативный фреймворк для написания тестов
- **K6** - современный инструмент для нагрузочного тестирования с поддержкой WebSocket

### Мониторинг тестов

Для непрерывного мониторинга качества:

1. Интегрируйте тесты в CI/CD pipeline (GitHub Actions, Jenkins, GitLab CI)
2. Настройте автоматическое выполнение тестов при каждом коммите
3. Используйте системы мониторинга для отслеживания производительности в реальном времени

### Чек-лист для тестирования WebSocket сервера

✅ Проверка установки соединения  
✅ Проверка отправки/получения сообщений  
✅ Проверка подтверждений доставки  
✅ Проверка статусов пользователей  
✅ Проверка обработки ошибок  
✅ Проверка повторного подключения  
✅ Проверка работы с большим количеством подключений  
✅ Проверка долговременной стабильности  
✅ Проверка работы с разными клиентами (браузеры, мобильные приложения)  

Следуя этим рекомендациям, вы сможете обеспечить надежное и стабильное функционирование WebSocket сервера чатов в различных условиях. 