// API и WebSocket коммуникация
class API {
    // Получение списка чатов для пользователя
    static async getChats() {
        try {
            const response = await fetch(`${CONFIG.API_URL}/api/chats?userId=${CONFIG.CURRENT_USER_ID}`);
            if (!response.ok) {
                throw new Error(`Ошибка HTTP: ${response.status}`);
            }
            const data = await response.json();
            return data.chats || [];
        } catch (error) {
            log('Ошибка при получении списка чатов:', error);
            return [];
        }
    }

    // Получение сообщений для конкретного чата
    static async getMessages(chatWithId) {
        try {
            const response = await fetch(`${CONFIG.API_URL}/api/messages?userId=${CONFIG.CURRENT_USER_ID}&chatWith=${chatWithId}`);
            if (!response.ok) {
                throw new Error(`Ошибка HTTP: ${response.status}`);
            }
            const data = await response.json();
            return data.messages || [];
        } catch (error) {
            log('Ошибка при получении сообщений:', error);
            return [];
        }
    }

    // Получение информации о пользователе по ID
    static async getUserInfo(userId) {
        try {
            const response = await fetch(`${CONFIG.API_URL}/api/users/${userId}`);
            if (!response.ok) {
                throw new Error(`Ошибка HTTP: ${response.status}`);
            }
            return await response.json();
        } catch (error) {
            log('Ошибка при получении информации о пользователе:', error);
            return null;
        }
    }

    // Получение информации о товаре по ID
    static async getProductInfo(productId) {
        try {
            const response = await fetch(`${CONFIG.API_URL}/api/products/${productId}`);
            if (!response.ok) {
                throw new Error(`Ошибка HTTP: ${response.status}`);
            }
            return await response.json();
        } catch (error) {
            log('Ошибка при получении информации о товаре:', error);
            return null;
        }
    }

    // Обновление статуса пользователя
    static async updateStatus(status = 'online') {
        try {
            const response = await fetch(`${CONFIG.API_URL}/api/status`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    userId: CONFIG.CURRENT_USER_ID,
                    status
                })
            });
            return response.ok;
        } catch (error) {
            log('Ошибка при обновлении статуса:', error);
            return false;
        }
    }
}

// Класс для работы с WebSocket
class ChatSocket {
    constructor() {
        this.socket = null;
        this.connected = false;
        this.onMessageCallback = null;
        this.onStatusChangeCallback = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectTimeout = 1000; // начальный таймаут 1 секунда
    }

    // Инициализация WebSocket соединения
    init() {
        if (this.socket && (this.socket.readyState === WebSocket.OPEN || this.socket.readyState === WebSocket.CONNECTING)) {
            return;
        }

        try {
            this.socket = new WebSocket(`${CONFIG.WS_URL}/${CONFIG.CURRENT_USER_ID}`);
            
            this.socket.onopen = () => {
                log('WebSocket соединение установлено');
                this.connected = true;
                this.reconnectAttempts = 0;
                
                // Отправляем статус онлайн при подключении
                this.sendStatus('online');
            };
            
            this.socket.onmessage = (event) => {
                const data = JSON.parse(event.data);
                log('Получены данные через WebSocket:', data);
                
                if (data.type === 'message' && this.onMessageCallback) {
                    this.onMessageCallback(data);
                } else if (data.type === 'status' && this.onStatusChangeCallback) {
                    this.onStatusChangeCallback(data);
                }
            };
            
            this.socket.onclose = (event) => {
                log('WebSocket соединение закрыто:', event);
                this.connected = false;
                
                // Пытаемся переподключиться
                if (this.reconnectAttempts < this.maxReconnectAttempts) {
                    this.reconnectAttempts++;
                    const timeout = this.reconnectTimeout * Math.pow(2, this.reconnectAttempts - 1);
                    log(`Попытка переподключения ${this.reconnectAttempts} через ${timeout}мс`);
                    
                    setTimeout(() => {
                        this.init();
                    }, timeout);
                }
            };
            
            this.socket.onerror = (error) => {
                log('Ошибка WebSocket:', error);
            };
            
        } catch (error) {
            log('Ошибка при инициализации WebSocket:', error);
        }
    }

    // Отправка сообщения через WebSocket
    sendMessage(to, content, productId) {
        if (!this.connected || !this.socket) {
            log('WebSocket не подключен. Невозможно отправить сообщение.');
            return false;
        }
        
        try {
            const message = {
                type: 'message',
                fromId: CONFIG.CURRENT_USER_ID,
                toId: to,
                productId: productId,
                content
            };
            
            this.socket.send(JSON.stringify(message));
            return true;
        } catch (error) {
            log('Ошибка при отправке сообщения через WebSocket:', error);
            return false;
        }
    }

    // Отправка статуса через WebSocket
    sendStatus(status) {
        if (!this.connected || !this.socket) {
            log('WebSocket не подключен. Невозможно отправить статус.');
            return false;
        }
        
        try {
            const statusMessage = {
                type: 'status',
                userId: CONFIG.CURRENT_USER_ID,
                status
            };
            
            this.socket.send(JSON.stringify(statusMessage));
            return true;
        } catch (error) {
            log('Ошибка при отправке статуса через WebSocket:', error);
            return false;
        }
    }

    // Установка обработчика для входящих сообщений
    onMessage(callback) {
        this.onMessageCallback = callback;
    }

    // Установка обработчика для изменения статуса
    onStatusChange(callback) {
        this.onStatusChangeCallback = callback;
    }

    // Закрытие соединения
    close() {
        if (this.socket) {
            this.sendStatus('offline');
            
            // Даем время на отправку статуса перед закрытием
            setTimeout(() => {
                this.socket.close();
                this.socket = null;
                this.connected = false;
            }, 100);
        }
    }
}

// Экспортируем экземпляр ChatSocket
const chatSocket = new ChatSocket(); 