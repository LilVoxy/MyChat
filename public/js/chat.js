// Основной модуль чата
class ChatApp {
    constructor() {
        // DOM элементы
        this.chatsList = document.getElementById('chats-container');
        this.chatMessages = document.getElementById('chat-messages');
        this.chatHeader = document.getElementById('chat-header');
        this.messageInput = document.getElementById('message-input');
        this.sendButton = document.getElementById('send-button');
        
        // Шаблоны
        this.chatItemTemplate = document.getElementById('chat-item-template');
        this.messageTemplate = document.getElementById('message-template');
        this.headerTemplate = document.getElementById('active-chat-header-template');
        
        // Состояние приложения
        this.chats = [];
        this.activeChat = null;
        this.userStatuses = {}; // Статусы пользователей: {userId: 'online'|'offline'}
        this.currentUser = CONFIG.CURRENT_USER_ID;
        
        // Кэши данных
        this.userInfoCache = {}; // {userId: {name, avatar}}
        this.productInfoCache = {}; // {productId: {name, image}}
        
        // Интервалы и таймеры
        this.statusUpdateInterval = null;
        this.chatUpdateInterval = null;
        
        // Привязка методов к this
        this.onChatItemClick = this.onChatItemClick.bind(this);
        this.onSendButtonClick = this.onSendButtonClick.bind(this);
        this.onMessageInputKeydown = this.onMessageInputKeydown.bind(this);
        this.onMessage = this.onMessage.bind(this);
        this.onStatusChange = this.onStatusChange.bind(this);
        this.loadChats = this.loadChats.bind(this);
        this.renderChats = this.renderChats.bind(this);
    }
    
    // Инициализация приложения
    async init() {
        log('Инициализация приложения чата');
        
        // Установка обработчиков событий для элементов формы
        this.sendButton.addEventListener('click', this.onSendButtonClick);
        this.messageInput.addEventListener('keydown', this.onMessageInputKeydown);
        
        // Инициализация WebSocket
        chatSocket.onMessage(this.onMessage);
        chatSocket.onStatusChange(this.onStatusChange);
        chatSocket.init();
        
        // Загрузка и отображение списка чатов
        await this.loadChats();
        
        // Установка интервала для обновления чатов
        this.chatUpdateInterval = setInterval(this.loadChats, 30000); // 30 секунд
        
        // Установка интервала для обновления статуса пользователя
        this.statusUpdateInterval = setInterval(() => {
            API.updateStatus('online');
        }, CONFIG.STATUS_INTERVAL);
        
        // Обработка закрытия окна
        window.addEventListener('beforeunload', () => {
            chatSocket.sendStatus('offline');
            clearInterval(this.statusUpdateInterval);
            clearInterval(this.chatUpdateInterval);
        });
        
        log('Инициализация завершена');
    }
    
    // Загрузка списка чатов
    async loadChats() {
        try {
            const chats = await API.getChats();
            if (chats && chats.length > 0) {
                this.chats = chats;
                this.renderChats();
                
                // Если есть активный чат, обновляем его данные
                if (this.activeChat) {
                    const updatedChat = this.chats.find(chat => 
                        (chat.buyerId === this.activeChat.buyerId && chat.sellerId === this.activeChat.sellerId) ||
                        (chat.buyerId === this.activeChat.sellerId && chat.sellerId === this.activeChat.buyerId)
                    );
                    
                    if (updatedChat) {
                        this.activeChat = updatedChat;
                    }
                }
            } else {
                log('Чаты не найдены или список пуст');
            }
        } catch (error) {
            log('Ошибка при загрузке чатов:', error);
        }
    }
    
    // Отображение списка чатов
    async renderChats() {
        // Очищаем контейнер
        this.chatsList.innerHTML = '';
        
        // Подготавливаем фрагмент для вставки
        const fragment = document.createDocumentFragment();
        
        // Для каждого чата получаем данные о собеседнике и товаре
        for (const chat of this.chats) {
            // Определяем ID собеседника (продавца или покупателя)
            const otherUserId = chat.buyerId === this.currentUser ? chat.sellerId : chat.buyerId;
            
            // Клонируем шаблон чата
            const chatItem = this.chatItemTemplate.content.cloneNode(true).querySelector('.chat-item');
            
            // Устанавливаем атрибуты данных
            chatItem.dataset.chatId = chat.id;
            chatItem.dataset.productId = chat.productId;
            chatItem.dataset.sellerId = chat.sellerId;
            chatItem.dataset.buyerId = chat.buyerId;
            
            // Находим элементы чата
            const name = chatItem.querySelector('.chat-item-name');
            const time = chatItem.querySelector('.chat-item-time');
            const message = chatItem.querySelector('.chat-item-message');
            const avatar = chatItem.querySelector('.chat-item-avatar img');
            const statusIndicator = chatItem.querySelector('.status-indicator');
            const productImage = chatItem.querySelector('.chat-item-product img');
            
            // Проверяем, активен ли этот чат
            if (this.activeChat && this.activeChat.id === chat.id) {
                chatItem.classList.add('active');
            }
            
            // Устанавливаем доступные данные
            time.textContent = chat.lastMessageTime;
            message.textContent = chat.lastMessage || 'Нет сообщений';
            
            // Загружаем и устанавливаем данные о пользователе
            let userInfo = this.userInfoCache[otherUserId];
            if (!userInfo) {
                try {
                    userInfo = await API.getUserInfo(otherUserId);
                    if (userInfo) {
                        this.userInfoCache[otherUserId] = userInfo;
                    }
                } catch (error) {
                    log('Ошибка при загрузке данных пользователя:', error);
                    userInfo = { name: `Пользователь ${otherUserId}`, avatar: CONFIG.DEFAULT_AVATAR };
                }
            }
            
            // Загружаем и устанавливаем данные о товаре
            let productInfo = this.productInfoCache[chat.productId];
            if (!productInfo) {
                try {
                    productInfo = await API.getProductInfo(chat.productId);
                    if (productInfo) {
                        this.productInfoCache[chat.productId] = productInfo;
                    }
                } catch (error) {
                    log('Ошибка при загрузке данных товара:', error);
                    productInfo = { name: `Товар ${chat.productId}`, image: CONFIG.DEFAULT_PRODUCT_IMAGE };
                }
            }
            
            // Устанавливаем имя пользователя
            name.textContent = userInfo?.name || `Пользователь ${otherUserId}`;
            
            // Устанавливаем аватар пользователя
            avatar.src = userInfo?.avatar || CONFIG.DEFAULT_AVATAR;
            avatar.alt = userInfo?.name || `Пользователь ${otherUserId}`;
            
            // Устанавливаем изображение товара
            productImage.src = productInfo?.image || CONFIG.DEFAULT_PRODUCT_IMAGE;
            productImage.alt = productInfo?.name || `Товар ${chat.productId}`;
            
            // Устанавливаем статус пользователя
            const status = this.userStatuses[otherUserId] || 'offline';
            statusIndicator.className = 'status-indicator';
            statusIndicator.classList.add(`status-${status}`);
            
            // Добавляем обработчик клика для открытия чата
            chatItem.addEventListener('click', () => this.onChatItemClick(chat, otherUserId));
            
            // Добавляем чат во фрагмент
            fragment.appendChild(chatItem);
        }
        
        // Вставляем все чаты в DOM
        this.chatsList.appendChild(fragment);
    }
    
    // Обработчик клика по элементу чата
    async onChatItemClick(chat, otherUserId) {
        // Если чат уже активен, не делаем ничего
        if (this.activeChat && this.activeChat.id === chat.id) {
            return;
        }
        
        // Обновляем активный чат
        this.activeChat = chat;
        
        // Обновляем UI - выделяем активный чат
        const chatItems = this.chatsList.querySelectorAll('.chat-item');
        chatItems.forEach(item => {
            item.classList.remove('active');
            if (Number(item.dataset.chatId) === chat.id) {
                item.classList.add('active');
            }
        });
        
        // Очищаем область сообщений
        this.chatMessages.innerHTML = '';
        
        // Очищаем заголовок чата
        this.chatHeader.innerHTML = '';
        
        // Включаем элементы ввода
        this.messageInput.disabled = false;
        this.sendButton.disabled = false;
        
        // Загружаем информацию о собеседнике
        let userInfo = this.userInfoCache[otherUserId];
        if (!userInfo) {
            try {
                userInfo = await API.getUserInfo(otherUserId);
                if (userInfo) {
                    this.userInfoCache[otherUserId] = userInfo;
                }
            } catch (error) {
                log('Ошибка при загрузке данных пользователя:', error);
                userInfo = { name: `Пользователь ${otherUserId}`, avatar: CONFIG.DEFAULT_AVATAR };
            }
        }
        
        // Загружаем информацию о товаре
        let productInfo = this.productInfoCache[chat.productId];
        if (!productInfo) {
            try {
                productInfo = await API.getProductInfo(chat.productId);
                if (productInfo) {
                    this.productInfoCache[chat.productId] = productInfo;
                }
            } catch (error) {
                log('Ошибка при загрузке данных товара:', error);
                productInfo = { name: `Товар ${chat.productId}`, image: CONFIG.DEFAULT_PRODUCT_IMAGE };
            }
        }
        
        // Создаем и настраиваем заголовок чата
        const headerElement = this.headerTemplate.content.cloneNode(true).querySelector('.active-chat-header');
        
        // Заполняем информацию о продавце
        const sellerName = headerElement.querySelector('.seller-name');
        const sellerAvatar = headerElement.querySelector('.seller-avatar img');
        const sellerStatus = headerElement.querySelector('.seller-status');
        const statusIndicator = headerElement.querySelector('.status-indicator');
        
        sellerName.textContent = userInfo?.name || `Пользователь ${otherUserId}`;
        sellerAvatar.src = userInfo?.avatar || CONFIG.DEFAULT_AVATAR;
        sellerAvatar.alt = userInfo?.name || `Пользователь ${otherUserId}`;
        
        // Устанавливаем статус
        const status = this.userStatuses[otherUserId] || 'offline';
        statusIndicator.className = 'status-indicator';
        statusIndicator.classList.add(`status-${status}`);
        sellerStatus.textContent = status === 'online' ? 'В сети' : 'Не в сети';
        
        // Заполняем информацию о товаре
        const productName = headerElement.querySelector('.product-name');
        const productImage = headerElement.querySelector('.product-image img');
        
        productName.textContent = productInfo?.name || `Товар ${chat.productId}`;
        productImage.src = productInfo?.image || CONFIG.DEFAULT_PRODUCT_IMAGE;
        productImage.alt = productInfo?.name || `Товар ${chat.productId}`;
        
        // Добавляем заголовок в DOM
        this.chatHeader.appendChild(headerElement);
        
        // Загружаем и отображаем сообщения
        await this.loadMessages(otherUserId);
    }
    
    // Загрузка сообщений для активного чата
    async loadMessages(otherUserId) {
        if (!this.activeChat) {
            return;
        }
        
        try {
            const messages = await API.getMessages(otherUserId);
            
            // Очищаем область сообщений
            this.chatMessages.innerHTML = '';
            
            // Подготавливаем фрагмент для вставки
            const fragment = document.createDocumentFragment();
            
            // Добавляем все сообщения
            for (const msg of messages) {
                const messageElement = this.createMessageElement(msg);
                fragment.appendChild(messageElement);
            }
            
            // Вставляем все сообщения в DOM
            this.chatMessages.appendChild(fragment);
            
            // Прокручиваем чат вниз
            setTimeout(() => {
                this.chatMessages.scrollTop = this.chatMessages.scrollHeight;
            }, CONFIG.SCROLL_DELAY);
            
        } catch (error) {
            log('Ошибка при загрузке сообщений:', error);
        }
    }
    
    // Создание элемента сообщения
    createMessageElement(message) {
        // Клонируем шаблон сообщения
        const messageElement = this.messageTemplate.content.cloneNode(true).querySelector('.message');
        
        // Находим элементы сообщения
        const content = messageElement.querySelector('.message-content');
        const time = messageElement.querySelector('.message-time');
        
        // Устанавливаем направление сообщения
        if (message.fromId === this.currentUser) {
            messageElement.classList.add('message-outgoing');
        } else {
            messageElement.classList.add('message-incoming');
        }
        
        // Устанавливаем содержимое и время
        content.textContent = message.content;
        time.textContent = message.timestamp;
        
        return messageElement;
    }
    
    // Добавление нового сообщения в интерфейс
    addMessage(message) {
        // Проверяем, соответствует ли сообщение активному чату
        const isCurrentChat = this.activeChat && 
            ((message.fromId === this.currentUser && message.toId === this.getActiveInterlocutorId()) || 
             (message.fromId === this.getActiveInterlocutorId() && message.toId === this.currentUser));
        
        if (isCurrentChat) {
            // Создаем и добавляем элемент сообщения
            const messageElement = this.createMessageElement(message);
            this.chatMessages.appendChild(messageElement);
            
            // Прокручиваем чат вниз
            setTimeout(() => {
                this.chatMessages.scrollTop = this.chatMessages.scrollHeight;
            }, CONFIG.SCROLL_DELAY);
        }
        
        // Обновляем список чатов
        this.loadChats();
    }
    
    // Получение ID собеседника в активном чате
    getActiveInterlocutorId() {
        if (!this.activeChat) {
            return null;
        }
        
        return this.activeChat.buyerId === this.currentUser 
            ? this.activeChat.sellerId 
            : this.activeChat.buyerId;
    }
    
    // Отправка сообщения
    sendMessage() {
        if (!this.activeChat || !this.messageInput.value.trim()) {
            return;
        }
        
        const content = this.messageInput.value.trim();
        const toUserId = this.getActiveInterlocutorId();
        
        // Отправляем сообщение через WebSocket
        const success = chatSocket.sendMessage(toUserId, content, this.activeChat.productId);
        
        if (success) {
            // Очищаем поле ввода
            this.messageInput.value = '';
            
            // Создаем объект сообщения для локального добавления
            const message = {
                fromId: this.currentUser,
                toId: toUserId,
                productId: this.activeChat.productId,
                content: content,
                timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
            };
            
            // Добавляем сообщение в интерфейс
            this.addMessage(message);
        }
    }
    
    // Обработчик события входящего сообщения через WebSocket
    onMessage(message) {
        if (message.type !== 'message') {
            return;
        }
        
        // Добавляем время сообщения, если его нет
        if (!message.timestamp) {
            message.timestamp = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        }
        
        // Добавляем сообщение в интерфейс
        this.addMessage(message);
        
        // Обновляем список чатов
        this.loadChats();
    }
    
    // Обработчик события изменения статуса пользователя
    onStatusChange(data) {
        if (data.type !== 'status' || !data.userId) {
            return;
        }
        
        // Обновляем статус в локальном хранилище
        this.userStatuses[data.userId] = data.status;
        
        // Обновляем отображение статуса в списке чатов
        const chatItems = this.chatsList.querySelectorAll('.chat-item');
        for (const item of chatItems) {
            const sellerId = Number(item.dataset.sellerId);
            const buyerId = Number(item.dataset.buyerId);
            
            if (data.userId === sellerId || data.userId === buyerId) {
                const statusIndicator = item.querySelector('.status-indicator');
                statusIndicator.className = 'status-indicator';
                statusIndicator.classList.add(`status-${data.status}`);
            }
        }
        
        // Обновляем статус в заголовке активного чата
        if (this.activeChat) {
            const interlocutorId = this.getActiveInterlocutorId();
            
            if (data.userId === interlocutorId) {
                const statusIndicator = this.chatHeader.querySelector('.status-indicator');
                const statusText = this.chatHeader.querySelector('.seller-status');
                
                if (statusIndicator && statusText) {
                    statusIndicator.className = 'status-indicator';
                    statusIndicator.classList.add(`status-${data.status}`);
                    statusText.textContent = data.status === 'online' ? 'В сети' : 'Не в сети';
                }
            }
        }
    }
    
    // Обработчик клика на кнопку отправки
    onSendButtonClick() {
        this.sendMessage();
    }
    
    // Обработчик нажатия клавиш в поле ввода
    onMessageInputKeydown(event) {
        // Отправка сообщения по Enter (без Shift)
        if (event.key === 'Enter' && !event.shiftKey) {
            event.preventDefault();
            this.sendMessage();
        }
    }
}

// Создаем и инициализируем приложение при загрузке страницы
document.addEventListener('DOMContentLoaded', () => {
    // Создаем заглушки для изображений
    createPlaceholderImages();
    
    // Создаем и инициализируем приложение
    const app = new ChatApp();
    app.init();
});

// Функция для создания заглушек для изображений
function createPlaceholderImages() {
    // Создаем заглушку аватара пользователя
    const avatarImage = new Image();
    avatarImage.src = 'data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 100 100"><rect width="100" height="100" fill="%23e0e0e0"/><circle cx="50" cy="40" r="20" fill="%23bdbdbd"/><rect x="25" y="70" width="50" height="30" rx="15" fill="%23bdbdbd"/></svg>';
    avatarImage.onload = () => {
        CONFIG.DEFAULT_AVATAR = avatarImage.src;
    };
    
    // Создаем заглушку изображения товара
    const productImage = new Image();
    productImage.src = 'data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 100 100"><rect width="100" height="100" fill="%23f5f5f5"/><rect x="25" y="25" width="50" height="50" fill="%23e0e0e0"/><path d="M35,40 L45,60 L55,45 L65,60" stroke="%23bdbdbd" stroke-width="2" fill="none"/><circle cx="40" cy="35" r="5" fill="%23bdbdbd"/></svg>';
    productImage.onload = () => {
        CONFIG.DEFAULT_PRODUCT_IMAGE = productImage.src;
    };
} 