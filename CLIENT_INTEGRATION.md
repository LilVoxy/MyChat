# Интеграция клиентского кода WebSocket

## Содержание

- [Изменения в API](#изменения-в-api)
- [Обновление клиентского JavaScript-кода](#обновление-клиентского-javascript-кода)
  - [Базовый шаблон обработчика сообщений](#базовый-шаблон-обработчика-сообщений)
- [Реализация для React](#реализация-для-react)
- [Примечания по реализации](#примечания-по-реализации)
- [Общая схема взаимодействия](#общая-схема-взаимодействия)
- [Обработка ошибок](#обработка-ошибок)
- [Оптимизация для мобильных клиентов](#оптимизация-для-мобильных-клиентов)
- [Часто задаваемые вопросы](#часто-задаваемые-вопросы)

После обновления серверной части WebSocket, необходимо адаптировать клиентский код для корректной обработки сообщений и избежания дублирования.

## Изменения в API

Ключевые изменения в протоколе WebSocket:

1. **Сообщения от пользователя больше не возвращаются обратно в полном виде**
2. **Вместо этого сервер отправляет подтверждение с типом 'confirmation'**
3. **Сообщения содержат ID, который можно использовать для связывания с подтверждениями**

## Обновление клиентского JavaScript-кода

### Базовый шаблон обработчика сообщений

```javascript
// Подключение к WebSocket
const userId = 123; // ID текущего пользователя
const ws = new WebSocket(`ws://localhost:8080/ws/${userId}`);

// Хранилище отправленных сообщений, ожидающих подтверждения
const pendingMessages = new Map();

// Хранилище сообщений для отображения в UI
const messages = [];

// Обработчик входящих сообщений
ws.onmessage = function(event) {
  const data = JSON.parse(event.data);
  
  // Обработка различных типов сообщений
  switch(data.type) {
    case 'message':
      // Входящее сообщение от другого пользователя
      console.log('Получено новое сообщение:', data);
      
      // Добавляем сообщение в хранилище и обновляем UI
      messages.push({
        id: data.id,
        fromId: data.fromId,
        toId: data.toId,
        content: data.content,
        timestamp: new Date(),
        status: 'received'
      });
      
      // Обновляем UI
      updateMessageUI();
      break;
      
    case 'confirmation':
      // Подтверждение отправленного сообщения
      console.log('Получено подтверждение для сообщения:', data.id);
      
      // Находим сообщение в ожидающих подтверждения
      const pendingMsg = pendingMessages.get(data.id);
      if (pendingMsg) {
        // Обновляем статус сообщения
        pendingMsg.status = data.status;
        pendingMsg.id = data.id; // Обновляем ID, полученный с сервера
        
        // Удаляем из ожидающих, так как подтверждение получено
        pendingMessages.delete(data.id);
        
        // Находим сообщение в основном массиве и обновляем его
        const msgIndex = messages.findIndex(m => m.tempId === pendingMsg.tempId);
        if (msgIndex >= 0) {
          messages[msgIndex] = {
            ...messages[msgIndex],
            id: data.id,
            status: data.status
          };
        }
        
        // Обновляем UI
        updateMessageUI();
      }
      break;
      
    case 'error':
      // Обработка ошибок
      console.error('Ошибка:', data.content);
      
      // Можно обновить UI, показав сообщение об ошибке
      showErrorMessage(data.content);
      break;
      
    case 'status':
      // Обработка изменений статуса пользователя
      console.log(`Пользователь ${data.userId} сейчас ${data.status}`);
      
      // Обновляем статус пользователя в UI
      updateUserStatus(data.userId, data.status);
      break;
  }
};

// Функция отправки сообщения
function sendMessage(toId, productId, content) {
  // Создаем временный ID для отслеживания сообщения до получения подтверждения
  const tempId = `temp_${Date.now()}`;
  
  // Создаем сообщение
  const message = {
    type: 'message',
    fromId: userId, // ID текущего пользователя
    toId: toId,
    productId: productId,
    content: content,
    tempId: tempId // Временный ID для отслеживания
  };
  
  // Добавляем сообщение в список отправленных, ожидающих подтверждения
  const pendingMessage = {
    ...message,
    status: 'sending',
    timestamp: new Date()
  };
  
  // Сохраняем во временное хранилище по временному ID
  pendingMessages.set(tempId, pendingMessage);
  
  // Также добавляем в основной массив сообщений для отображения
  messages.push(pendingMessage);
  
  // Обновляем UI, показывая сообщение как "отправляемое"
  updateMessageUI();
  
  // Отправляем сообщение на сервер
  ws.send(JSON.stringify(message));
  
  console.log('Сообщение отправлено, ожидаем подтверждения:', pendingMessage);
}

// Пример функции обновления UI (должна быть реализована в соответствии с вашим UI)
function updateMessageUI() {
  // Этот код следует адаптировать под ваш UI-фреймворк (React, Vue, etc.)
  const chatContainer = document.getElementById('chat-messages');
  
  // Очищаем контейнер
  chatContainer.innerHTML = '';
  
  // Сортируем сообщения по времени
  const sortedMessages = [...messages].sort((a, b) => a.timestamp - b.timestamp);
  
  // Добавляем сообщения в контейнер
  sortedMessages.forEach(msg => {
    const messageElement = document.createElement('div');
    messageElement.className = `message ${msg.fromId === userId ? 'outgoing' : 'incoming'}`;
    
    // Добавляем индикатор статуса для исходящих сообщений
    let statusIndicator = '';
    if (msg.fromId === userId) {
      if (msg.status === 'sending') statusIndicator = ' (Отправляется...)';
      else if (msg.status === 'sent') statusIndicator = ' (Отправлено)';
      else if (msg.status === 'delivered') statusIndicator = ' (Доставлено)';
      else if (msg.status === 'read') statusIndicator = ' (Прочитано)';
    }
    
    messageElement.innerHTML = `
      <div class="message-content">${msg.content}</div>
      <div class="message-time">${msg.timestamp.toLocaleTimeString()}${statusIndicator}</div>
    `;
    
    chatContainer.appendChild(messageElement);
  });
  
  // Прокручиваем чат вниз
  chatContainer.scrollTop = chatContainer.scrollHeight;
}

// Функция отображения ошибок
function showErrorMessage(errorText) {
  const errorElement = document.createElement('div');
  errorElement.className = 'error-message';
  errorElement.textContent = errorText;
  
  document.getElementById('chat-container').appendChild(errorElement);
  
  // Автоматически удаляем сообщение об ошибке через 5 секунд
  setTimeout(() => {
    errorElement.remove();
  }, 5000);
}

// Функция обновления статуса пользователя в UI
function updateUserStatus(userId, status) {
  const userElement = document.querySelector(`.user[data-id="${userId}"]`);
  if (userElement) {
    userElement.querySelector('.status').textContent = status;
    userElement.setAttribute('data-status', status);
  }
}
```

## Реализация для React

Если вы используете React, вот пример компонента чата:

```jsx
import React, { useState, useEffect, useRef } from 'react';

function Chat({ currentUserId, recipientId, productId }) {
  const [messages, setMessages] = useState([]);
  const [pendingMessages, setPendingMessages] = useState(new Map());
  const [inputText, setInputText] = useState('');
  const wsRef = useRef(null);
  
  // Инициализация WebSocket при монтировании компонента
  useEffect(() => {
    // Создаем соединение
    wsRef.current = new WebSocket(`ws://localhost:8080/ws/${currentUserId}`);
    
    // Обработчик входящих сообщений
    wsRef.current.onmessage = (event) => {
      const data = JSON.parse(event.data);
      
      switch(data.type) {
        case 'message':
          // Новое входящее сообщение
          setMessages(prevMessages => [
            ...prevMessages,
            {
              id: data.id,
              fromId: data.fromId,
              toId: data.toId,
              content: data.content,
              timestamp: new Date(),
              status: 'received'
            }
          ]);
          break;
          
        case 'confirmation':
          // Подтверждение отправленного сообщения
          setPendingMessages(prev => {
            const newMap = new Map(prev);
            newMap.delete(data.id);
            return newMap;
          });
          
          setMessages(prevMessages => 
            prevMessages.map(msg => 
              (msg.tempId === data.tempId) 
                ? { ...msg, id: data.id, status: data.status } 
                : msg
            )
          );
          break;
          
        case 'error':
          console.error('Ошибка:', data.content);
          // Здесь можно добавить отображение ошибки в UI
          break;
          
        case 'status':
          // Обработка изменений статуса пользователя
          // Может быть реализована через глобальное состояние или контекст
          break;
      }
    };
    
    // Очистка при размонтировании
    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [currentUserId]);
  
  // Функция отправки сообщения
  const sendMessage = () => {
    if (!inputText.trim()) return;
    
    // Генерируем временный ID
    const tempId = `temp_${Date.now()}`;
    
    // Создаем сообщение
    const message = {
      type: 'message',
      fromId: currentUserId,
      toId: recipientId,
      productId: productId,
      content: inputText,
      tempId: tempId
    };
    
    // Добавляем в список отправленных
    const pendingMessage = {
      ...message,
      status: 'sending',
      timestamp: new Date()
    };
    
    // Обновляем состояние
    setPendingMessages(prev => {
      const newMap = new Map(prev);
      newMap.set(tempId, pendingMessage);
      return newMap;
    });
    
    setMessages(prev => [...prev, pendingMessage]);
    
    // Отправляем сообщение
    wsRef.current.send(JSON.stringify(message));
    
    // Очищаем поле ввода
    setInputText('');
  };
  
  return (
    <div className="chat-container">
      <div className="messages-list">
        {messages
          .sort((a, b) => a.timestamp - b.timestamp)
          .map((msg) => (
            <div 
              key={msg.id || msg.tempId} 
              className={`message ${msg.fromId === currentUserId ? 'outgoing' : 'incoming'}`}
            >
              <div className="message-content">{msg.content}</div>
              <div className="message-info">
                {msg.timestamp.toLocaleTimeString()}
                {msg.fromId === currentUserId && (
                  <span className="status-indicator">
                    {msg.status === 'sending' && ' ⏳'}
                    {msg.status === 'sent' && ' ✓'}
                    {msg.status === 'delivered' && ' ✓✓'}
                    {msg.status === 'read' && ' ✓✓✓'}
                  </span>
                )}
              </div>
            </div>
          ))}
      </div>
      
      <div className="message-input">
        <input
          type="text"
          value={inputText}
          onChange={(e) => setInputText(e.target.value)}
          onKeyPress={(e) => e.key === 'Enter' && sendMessage()}
          placeholder="Введите сообщение..."
        />
        <button onClick={sendMessage}>Отправить</button>
      </div>
    </div>
  );
}

export default Chat;
```

## Примечания по реализации

1. Обязательно обрабатывайте сообщения типа `confirmation` для обновления статуса отправленных сообщений
2. Используйте временные ID для отслеживания сообщений до получения подтверждения с сервера
3. Отображайте статус сообщения пользователю (отправляется, отправлено и т.д.)
4. Обрабатывайте ошибки отправки и другие уведомления от сервера

## Общая схема взаимодействия

1. Пользователь отправляет сообщение → клиент показывает сообщение со статусом "отправляется"
2. Сервер получает сообщение, сохраняет его в БД и возвращает подтверждение (не саму копию сообщения)
3. Клиент получает подтверждение и обновляет статус сообщения на "отправлено"
4. Получатель получает сообщение и отображает его в своем интерфейсе

Этот подход решает проблему дублирования сообщений и обеспечивает более надежное отслеживание статуса сообщений. 

## Обработка ошибок

### Типы ошибок

При работе с WebSocket соединением могут возникать следующие типы ошибок:

1. **Ошибки соединения** - проблемы с установкой или потерей соединения
2. **Ошибки формата сообщений** - неправильный JSON или неверные поля
3. **Ошибки авторизации** - проблемы с доступом или правами
4. **Ошибки бизнес-логики** - нарушение правил работы системы

### Стратегии обработки

#### Восстановление соединения

```javascript
function createWebSocket(userId) {
  let socket = new WebSocket(`ws://localhost:8080/ws/${userId}`);
  let reconnectAttempts = 0;
  const maxReconnectAttempts = 5;
  
  socket.onclose = function(event) {
    if (event.wasClean) {
      console.log('Соединение закрыто корректно');
    } else {
      console.error('Соединение разорвано');
      
      // Экспоненциальный backoff для повторных попыток
      if (reconnectAttempts < maxReconnectAttempts) {
        const timeout = Math.pow(2, reconnectAttempts) * 1000;
        console.log(`Повторное подключение через ${timeout / 1000} сек...`);
        
        setTimeout(() => {
          reconnectAttempts++;
          socket = createWebSocket(userId);
        }, timeout);
      } else {
        console.error('Превышено максимальное число попыток подключения');
        showConnectionError('Не удалось подключиться к серверу. Попробуйте позже.');
      }
    }
  };
  
  socket.onerror = function(error) {
    console.error('Ошибка WebSocket:', error);
  };
  
  return socket;
}

// Использование функции
const ws = createWebSocket(123);
```

#### Обработка ошибок сервера

```javascript
ws.onmessage = function(event) {
  const data = JSON.parse(event.data);
  
  if (data.type === 'error') {
    console.error('Ошибка от сервера:', data.content);
    
    switch(data.code) {
      case 'INVALID_MESSAGE':
        // Обработка ошибки формата сообщения
        showNotification('Ошибка формата сообщения', 'error');
        break;
        
      case 'MESSAGE_TOO_LARGE':
        // Обработка ошибки превышения размера сообщения
        showNotification('Сообщение слишком большое', 'error');
        break;
        
      case 'UNAUTHORIZED':
        // Обработка ошибки авторизации
        handleAuthError();
        break;
        
      default:
        // Обработка прочих ошибок
        showNotification('Произошла ошибка: ' + data.content, 'error');
    }
  }
};
```

### Отображение ошибок в UI

Рекомендуется создать единый компонент для отображения ошибок:

```javascript
function showNotification(message, type = 'info', duration = 5000) {
  const notification = document.createElement('div');
  notification.className = `notification notification-${type}`;
  notification.textContent = message;
  
  document.getElementById('notifications-container').appendChild(notification);
  
  setTimeout(() => {
    notification.classList.add('fadeout');
    setTimeout(() => notification.remove(), 300);
  }, duration);
}
```

## Оптимизация для мобильных клиентов

### Управление соединением

На мобильных устройствах важно оптимизировать использование батареи и трафика:

```javascript
// Флаг активности приложения
let isAppActive = true;

// Обработчики событий видимости приложения
document.addEventListener('visibilitychange', function() {
  isAppActive = !document.hidden;
  
  if (isAppActive) {
    // Приложение активно, восстанавливаем соединение
    if (ws.readyState !== WebSocket.OPEN) {
      ws = createWebSocket(userId);
    }
  } else {
    // Приложение неактивно, можно закрыть соединение для экономии ресурсов
    if (ws.readyState === WebSocket.OPEN) {
      // Отправляем статус "away" перед закрытием
      ws.send(JSON.stringify({
        type: 'status',
        userId: userId,
        status: 'away'
      }));
      
      // Закрываем соединение
      ws.close(1000, 'App inactive');
    }
  }
});

// Для мобильных приложений (React Native, Flutter, etc.)
// используйте соответствующие API для отслеживания состояния приложения
// например, AppState в React Native
```

### Экономия трафика

1. **Оптимизация размера сообщений**:
   - Используйте короткие имена полей
   - Отправляйте только необходимые данные

2. **Пакетная отправка**:
   - Группируйте события чтения сообщений
   - Отправляйте одно событие "read" вместо нескольких

```javascript
// Пример оптимизации отметки о прочтении
let lastReadIds = {};
let readTimeout = null;

function markAsRead(toId, productId, messageId) {
  // Сохраняем последний ID для каждой комбинации получатель+товар
  const key = `${toId}:${productId}`;
  lastReadIds[key] = messageId;
  
  // Откладываем отправку, чтобы сгруппировать несколько событий
  if (readTimeout) {
    clearTimeout(readTimeout);
  }
  
  readTimeout = setTimeout(() => {
    // Отправляем все накопленные отметки о прочтении
    for (const [key, id] of Object.entries(lastReadIds)) {
      const [toId, productId] = key.split(':');
      ws.send(JSON.stringify({
        type: 'read',
        fromId: userId,
        toId: parseInt(toId),
        productId: parseInt(productId),
        lastReadId: id
      }));
    }
    
    lastReadIds = {};
    readTimeout = null;
  }, 1000);
}
```

## Часто задаваемые вопросы

### 1. Как обрабатывать одновременные соединения с нескольких устройств?

Система поддерживает несколько одновременных соединений для одного пользователя. Сообщения будут доставлены на все устройства. Каждое устройство должно иметь уникальный идентификатор для отслеживания:

```javascript
const deviceId = localStorage.getItem('device_id') || generateUUID();
localStorage.setItem('device_id', deviceId);

// При подключении передаем дополнительный заголовок
const ws = new WebSocket(`ws://localhost:8080/ws/${userId}`);
ws.setRequestHeader('X-Device-ID', deviceId);
```

### 2. Что делать, если соединение часто разрывается?

- Используйте стратегию экспоненциального отката (exponential backoff)
- Проверьте качество сети
- Уменьшите частоту отправки сообщений
- Увеличьте таймаут пинг-понг сообщений

### 3. Как проверить состояние соединения перед отправкой?

```javascript
function safeSend(socket, message) {
  return new Promise((resolve, reject) => {
    if (socket.readyState === WebSocket.CONNECTING) {
      // Подключение еще не установлено
      socket.addEventListener('open', () => {
        socket.send(message);
        resolve();
      }, { once: true });
    } else if (socket.readyState === WebSocket.OPEN) {
      // Соединение активно
      socket.send(message);
      resolve();
    } else {
      // Соединение закрыто или закрывается
      reject(new Error('WebSocket connection is not open'));
    }
  });
}

// Использование
safeSend(ws, JSON.stringify({ type: 'message', ... }))
  .then(() => console.log('Сообщение отправлено'))
  .catch(error => {
    console.error('Ошибка отправки:', error);
    // Здесь можно попытаться переподключиться
  });
```

### 4. Как отобразить правильный статус сообщения?

- `sending` - Сообщение отправляется (на клиенте)
- `sent` - Сообщение доставлено на сервер
- `delivered` - Сообщение доставлено получателю (есть активное соединение)
- `read` - Сообщение прочитано получателем (получено уведомление "read")

Используйте иконки или текст для отображения статуса:

```javascript
function getStatusIcon(status) {
  switch(status) {
    case 'sending': return '⏳';
    case 'sent': return '✓';
    case 'delivered': return '✓✓';
    case 'read': return '✓✓✓';
    default: return '';
  }
}
``` 