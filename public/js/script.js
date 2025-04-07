document.addEventListener('DOMContentLoaded', function() {
    // Элементы интерфейса
    const messageInput = document.querySelector('.message-form input');
    const sendButton = document.querySelector('.message-form .btn.send');
    const messagesContainer = document.querySelector('.messages-container');
    const chatItems = document.querySelectorAll('.chat-item');
    const closeProductInfoButton = document.querySelector('.product-info .btn.close');
    const productInfoBtn = document.querySelector('.info-btn');
    const productInfo = document.querySelector('.product-info');
    const chatArea = document.querySelector('.chat-area');
    
    // Проверяем наличие основных элементов интерфейса
    if (!chatArea) {
        console.error('Не найден элемент .chat-area');
    }
    
    if (!messagesContainer) {
        console.error('Не найден элемент .messages-container');
    }
    
    // Элементы для выбора пользователя
    const userSelectOverlay = document.getElementById('userSelectOverlay');
    const userIdInput = document.getElementById('userIdInput');
    const startChatButton = document.getElementById('startChatButton');
    
    // При загрузке страницы автоматически показываем панель товара (если не мобильное устройство)
    if (window.innerWidth > 1200 && productInfo && chatArea) {
        productInfo.style.display = 'flex';
        chatArea.style.paddingRight = '300px';
    }
    
    // Элементы панели эмодзи
    const emojiButton = document.querySelector('.btn.emoji');
    const emojiPanel = document.querySelector('.emoji-panel');
    const emojiTabs = document.querySelectorAll('.emoji-tab');
    const emojiItems = document.querySelectorAll('.emoji-item');
    
    // Получаем ID пользователя из URL
    let userId = window.getUserIdFromUrl();
    
    // Если ID не определен, прерываем только инициализацию чата
    if (userId === null) {
        console.log('❌ ID пользователя не определен, чат не будет инициализирован');
        window.updateDebugPanel(null, 'не подключен');
        return;
    }
    
    console.log(`✅ Инициализация чата для пользователя с ID: ${userId}`);
    
    // Обновляем отладочную панель
    window.updateDebugPanel(userId, 'подключение...');
    
    // Определяем ID собеседника
    let currentChatUserId = userId === 1 ? 2 : 1;
    let currentProductId = 1;
    
    // Обновляем заголовок страницы
    document.title = `Чат - Пользователь ${userId}`;
    
    // Обновляем индикатор ID пользователя
    const userIdIndicator = document.createElement('div');
    userIdIndicator.classList.add('user-id-indicator');
    userIdIndicator.innerHTML = `<strong>Ваш ID:</strong> ${userId}`;
    userIdIndicator.style.position = 'absolute';
    userIdIndicator.style.top = '10px';
    userIdIndicator.style.right = '10px';
    userIdIndicator.style.backgroundColor = '#4caf50';
    userIdIndicator.style.color = 'white';
    userIdIndicator.style.padding = '5px 10px';
    userIdIndicator.style.borderRadius = '5px';
    userIdIndicator.style.zIndex = '1000';
    document.body.appendChild(userIdIndicator);
    
    // WebSocket соединение
    let socket;
    let reconnectAttempts = 0;
    const MAX_RECONNECT_ATTEMPTS = 5;
    
    // Добавляем константы для проверки соединения
    const PING_INTERVAL = 30000; // 30 секунд
    const PING_TIMEOUT = 5000; // 5 секунд
    let pingTimeout = null;
    let pingInterval = null;
    
    // Добавляем переменную для отслеживания намеренного закрытия
    let isIntentionalClose = false;
    
    // Добавляем константы для отслеживания активности
    const ACTIVITY_TIMEOUT = 60000; // 60 секунд
    const INACTIVITY_CHECK_INTERVAL = 30000; // 30 секунд
    const RECONNECT_DELAY = 5000; // 5 секунд
    let lastActivityTime = Date.now();
    let isUserActive = true;
    let currentStatus = 'offline';
    
    // Функция для обновления времени последней активности
    function updateLastActivity() {
        lastActivityTime = Date.now();
        if (!isUserActive) {
            isUserActive = true;
            updateStatus('online');
        }
    }
    
    // Функция обновления статуса
    function updateStatus(newStatus) {
        if (currentStatus !== newStatus) {
            currentStatus = newStatus;
            console.log(`🔄 Обновление статуса: ${newStatus}`);
            
            if (socket && socket.readyState === WebSocket.OPEN) {
                const statusMsg = {
                    type: 'status',
                    userId: userId,
                    status: newStatus,
                    isActive: isUserActive
                };
                socket.send(JSON.stringify(statusMsg));
            } else {
                // Если сокет недоступен, отправляем через HTTP
                fetch('/api/status', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        type: 'status',
                        userId: userId,
                        status: newStatus,
                        isActive: isUserActive
                    })
                }).catch(error => console.error('Ошибка отправки статуса:', error));
            }
        }
    }
    
    // Отслеживаем действия пользователя
    document.addEventListener('mousemove', updateLastActivity);
    document.addEventListener('keypress', updateLastActivity);
    document.addEventListener('click', updateLastActivity);
    document.addEventListener('scroll', updateLastActivity);
    document.addEventListener('touchstart', updateLastActivity);
    
    // Функция проверки активности пользователя
    function checkUserActivity() {
        const now = Date.now();
        if (now - lastActivityTime > ACTIVITY_TIMEOUT && isUserActive) {
            isUserActive = false;
            updateStatus('away');
        }
    }
    
    // Запускаем периодическую проверку активности
    setInterval(checkUserActivity, INACTIVITY_CHECK_INTERVAL);
    
    function initWebSocket() {
        if (socket) {
            socket.close();
        }

        socket = new WebSocket(`ws://${window.location.host}/ws/${userId}`);
        
        socket.onopen = function(e) {
            console.log(`✅ WebSocket соединение установлено для пользователя ${userId}`);
            reconnectAttempts = 0;
            window.updateDebugPanel(userId, 'подключен');

            // Запрашиваем актуальные статусы всех пользователей
            const statusRequest = {
                type: 'status_request',
                userId: userId
            };
            socket.send(JSON.stringify(statusRequest));

            // Отправляем свой статус с учетом активности
            updateStatus(isUserActive ? 'online' : 'away');

            // Устанавливаем интервал отправки пинг-сообщений
            pingInterval = setInterval(() => {
                if (socket.readyState === WebSocket.OPEN) {
                    socket.send(JSON.stringify({ 
                        type: 'ping',
                        userId: userId,
                        isActive: isUserActive
                    }));
                    pingTimeout = setTimeout(() => {
                        console.log('Пинг таймаут, переподключение...');
                        isIntentionalClose = true;
                        socket.close();
                    }, PING_TIMEOUT);
                }
            }, PING_INTERVAL);
        };

        socket.onclose = function(event) {
            console.log('WebSocket соединение закрыто');
            window.updateDebugPanel(userId, 'отключен');

            // Очищаем таймеры
            if (pingInterval) clearInterval(pingInterval);
            if (pingTimeout) clearTimeout(pingTimeout);

            // Обновляем статус на оффлайн при закрытии соединения
            updateStatus('offline');

            // Пытаемся переподключиться только если это не было намеренное закрытие
            if (!isIntentionalClose && reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
                reconnectAttempts++;
                console.log(`🔄 Попытка переподключения ${reconnectAttempts} из ${MAX_RECONNECT_ATTEMPTS}`);
                window.updateDebugPanel(userId, `переподключение (${reconnectAttempts}/${MAX_RECONNECT_ATTEMPTS})...`);
                setTimeout(initWebSocket, RECONNECT_DELAY);
            } else {
                window.updateDebugPanel(userId, isIntentionalClose ? 'отключен' : 'отключен (превышено число попыток)');
            }
        };

        socket.onmessage = function(event) {
            console.log(`📩 Получено сообщение через WebSocket:`, event.data);
            try {
                const messages = event.data.split('\n').filter(msg => msg.trim());
                messages.forEach(msg => {
                    try {
                        const data = JSON.parse(msg);
                        
                        // Обработка понг-сообщения
                        if (data.type === 'pong') {
                            if (pingTimeout) {
                                clearTimeout(pingTimeout);
                                pingTimeout = null;
                            }
                            return;
                        }

                        switch (data.type) {
                            case 'status':
                                console.log(`📊 Получено обновление статуса:`, data);
                                // Обновляем статус пользователя в списке чатов
                                updateUserStatus(data.userId, data.status);
                                break;

                            case 'message':
                                // Проверяем, относится ли сообщение к текущему чату
                                if ((data.fromId == currentChatUserId && data.toId == userId) || 
                                    (data.fromId == userId && data.toId == currentChatUserId)) {
                                    // Определяем, входящее или исходящее сообщение
                                    const isIncoming = data.fromId == currentChatUserId;
                                    
                                    // Отображаем сообщение в чате
                                    displayMessage(data.content, isIncoming, data.timestamp);
                                    
                                    // Обновляем последнее сообщение в списке чатов
                                    updateLastMessage(currentChatUserId, data.content);
                                } else if (data.toId == userId) {
                                    // Если сообщение из другого чата, обновляем счетчик непрочитанных
                                    incrementUnreadCount(data.fromId);
                                }
                                break;
                        }
                    } catch (e) {
                        console.error("❌ Ошибка при разборе JSON сообщения:", e);
                    }
                });
            } catch (e) {
                console.error("❌ Ошибка при обработке WebSocket сообщения:", e);
            }
        };
    }
    
    // Инициализируем WebSocket при загрузке страницы
    initWebSocket();
    
    // Загружаем список чатов пользователя
    loadUserChats();
    
    // Функция загрузки списка чатов пользователя
    async function loadUserChats() {
        try {
            const response = await fetch(`/api/chats?userId=${userId}`);
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            
            const data = await response.json();
            console.log('📬 Получены чаты:', data);
            
            // Очищаем список чатов
            const chatList = document.querySelector('.chat-list');
            if (!chatList) {
                console.error('Не найден элемент .chat-list');
                return;
            }
            
            chatList.innerHTML = '';
            
            // Проверяем, что data.chats существует и является массивом
            if (!data.chats || !Array.isArray(data.chats)) {
                console.error('Некорректные данные чатов:', data);
                chatList.innerHTML = '<div class="no-chats">Не удалось загрузить список чатов</div>';
                return;
            }
            
            // Добавляем каждый чат в список
            data.chats.forEach(chat => {
                // Проверяем, что все необходимые поля существуют
                if (!chat || chat.id === null || chat.id === undefined) {
                    console.warn('Некорректные данные чата:', chat);
                    return; // пропускаем этот чат
                }

                // Определяем ID собеседника (если текущий пользователь - покупатель, то берем продавца, и наоборот)
                const otherUserId = chat.buyerId == userId ? chat.sellerId : chat.buyerId;
                
                if (!otherUserId) {
                    console.warn('Не удалось определить ID собеседника для чата:', chat);
                    return; // пропускаем этот чат
                }
                
                // Создаем элемент чата
                const chatItem = document.createElement('div');
                chatItem.className = 'chat-item';
                chatItem.setAttribute('data-userId', otherUserId);
                chatItem.setAttribute('data-chatId', chat.id);
                
                // Добавляем обработчик клика
                chatItem.onclick = () => selectChat(otherUserId);
                
                // Форматируем HTML чата
                chatItem.innerHTML = `
                    <div class="chat-item-avatar">
                        <img src="/images/avatar.png" alt="User Avatar">
                        <span class="chat-item-status offline"></span>
                    </div>
                    <div class="chat-item-info">
                        <div class="chat-item-name">Пользователь ${otherUserId}</div>
                        <div class="chat-item-last-message">${chat.lastMessage || 'Нет сообщений'}</div>
                    </div>
                    <div class="chat-item-time">${chat.lastMessageTime || ''}</div>
                `;
                
                // Добавляем чат в список
                chatList.appendChild(chatItem);
            });
            
            // Если есть чаты, выбираем первый
            if (data.chats.length > 0) {
                try {
                    // Определяем ID собеседника первого чата
                    const firstChatUserId = data.chats[0].buyerId == userId ? data.chats[0].sellerId : data.chats[0].buyerId;
                    
                    // Устанавливаем текущего собеседника
                    currentChatUserId = firstChatUserId;
                    
                    // Выбираем первый чат
                    const firstChatItem = document.querySelector(`.chat-item[data-userId="${firstChatUserId}"]`);
                    if (firstChatItem) {
                        firstChatItem.classList.add('active');
                        updateChatHeader(firstChatItem); // Обновляем шапку чата
                        loadChatHistory(firstChatUserId); // Загружаем историю чата
                    } else {
                        console.warn(`Не найден элемент чата для пользователя с ID ${firstChatUserId}`);
                        updateChatHeader(null);
                    }
                } catch (innerError) {
                    console.error('Ошибка при выборе первого чата:', innerError);
                    updateChatHeader(null);
                }
            } else {
                // Если чатов нет, показываем сообщение
                chatList.innerHTML = '<div class="no-chats">У вас пока нет чатов</div>';
                
                // Устанавливаем заголовок чата на "Выберите чат"
                const titleElement = document.querySelector('.chat-header .user-info h3');
                if (titleElement) {
                    titleElement.textContent = 'Выберите чат';
                }
            }
            
        } catch (error) {
            console.error('❌ Ошибка при загрузке чатов:', error);
            const chatList = document.querySelector('.chat-list');
            if (chatList) {
                chatList.innerHTML = '<div class="error-message">Не удалось загрузить список чатов. Пожалуйста, попробуйте позже или обновите страницу</div>';
            }
            
            // Устанавливаем заголовок чата на "Выберите чат" в случае ошибки
            const titleElement = document.querySelector('.chat-header .user-info h3');
            if (titleElement) {
                titleElement.textContent = 'Выберите чат';
            }
        }
    }
    
    // Отправка сообщения через WebSocket
    function sendMessageToServer(text) {
        if (socket && socket.readyState === WebSocket.OPEN) {
            const message = {
                type: 'message',
                fromId: userId,
                toId: currentChatUserId,
                productId: currentProductId,
                content: text
            };
            
            socket.send(JSON.stringify(message));
            return true;
        } else {
            console.error("WebSocket соединение не установлено");
            return false;
        }
    }
    
    // Обновление последнего сообщения в списке чатов
    function updateLastMessage(chatUserId, text) {
        const chatItem = findChatItemByUserId(chatUserId);
        if (chatItem) {
            const lastMessageElement = chatItem.querySelector('.chat-item-last-message');
            if (lastMessageElement) {
                lastMessageElement.textContent = text;
            }
            
            // Обновляем время последнего сообщения
            const timeElement = chatItem.querySelector('.chat-item-time');
            if (timeElement) {
                const now = new Date();
                timeElement.textContent = `${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}`;
            }
        }
    }
    
    // Увеличение счетчика непрочитанных сообщений
    function incrementUnreadCount(fromUserId) {
        const chatItem = findChatItemByUserId(fromUserId);
        if (chatItem) {
            let unreadElement = chatItem.querySelector('.chat-item-unread');
            
            if (unreadElement) {
                // Увеличиваем значение счетчика
                let count = parseInt(unreadElement.textContent) || 0;
                unreadElement.textContent = count + 1;
            } else {
                // Создаем счетчик, если его нет
                const chatMeta = chatItem.querySelector('.chat-item-info');
                if (chatMeta) {
                    unreadElement = document.createElement('span');
                    unreadElement.className = 'chat-item-unread';
                    unreadElement.textContent = '1';
                    chatMeta.appendChild(unreadElement);
                }
            }
        }
    }
    
    // Поиск элемента чата по ID пользователя
    function findChatItemByUserId(userId) {
        return document.querySelector(`.chat-item[data-userId="${userId}"]`);
    }
    
    // Мобильная версия
    const isMobile = window.innerWidth <= 576;
    const chatList = document.querySelector('.chat-list');
    
    // Добавляем кнопку "Назад" для мобильной версии
    if (isMobile) {
        const backButton = document.createElement('button');
        backButton.classList.add('btn', 'back-btn');
        backButton.innerHTML = '<i class="fas fa-arrow-left"></i>';
        backButton.style.display = 'block';
        
        const headerActions = document.querySelector('.header-actions');
        headerActions.parentNode.insertBefore(backButton, headerActions);
        
        // Показываем чат-лист, скрываем область чата на мобильных
        chatList.classList.add('active');
        chatArea.classList.add('hidden');
        
        // Обработчик для кнопки "Назад"
        backButton.addEventListener('click', function() {
            chatList.classList.add('active');
            chatArea.classList.remove('active');
            chatArea.classList.add('hidden');
        });
    }
    
    // Обработчики событий для сообщений
    sendButton.addEventListener('click', sendMessage);
    messageInput.addEventListener('keypress', function(e) {
        if (e.key === 'Enter') {
            sendMessage();
        }
    });
    
    // Делегируем обработку кликов по чатам родительскому элементу
    chatList.addEventListener('click', function(e) {
        // Находим ближайший элемент .chat-item от места клика
        const chatItem = e.target.closest('.chat-item');
        if (chatItem) {
            // Удаляем активный класс у всех элементов
            document.querySelectorAll('.chat-item').forEach(chat => chat.classList.remove('active'));
            
            // Добавляем активный класс выбранному элементу
            chatItem.classList.add('active');
            
            // Обновляем текущего собеседника
            currentChatUserId = parseInt(chatItem.getAttribute('data-userId'));
            
            // Обновляем шапку чата с информацией о пользователе
            updateChatHeader(chatItem);
            
            // На мобильном: показываем область чата, скрываем список чатов
            if (isMobile) {
                chatList.classList.remove('active');
                chatArea.classList.add('active');
                chatArea.classList.remove('hidden');
            } else {
                // На десктопе: показываем информацию о товаре
                if (window.innerWidth > 1200) {
                    productInfo.style.display = 'flex';
                    chatArea.style.paddingRight = '300px';
                }
            }
            
            // Удаляем индикатор непрочитанных сообщений
            const unread = chatItem.querySelector('.chat-item-unread');
            if (unread) {
                unread.remove();
            }
            
            // Загружаем историю сообщений
            loadChatHistory(currentChatUserId);
        }
    });
    
    // Кнопка для открытия/закрытия информации о товаре
    if (productInfoBtn) {
        productInfoBtn.addEventListener('click', function() {
            // Проверяем, отображается ли уже панель
            if (productInfo.style.display === 'flex') {
                // Если панель уже отображается, скрываем её
                productInfo.style.display = 'none';
                chatArea.style.paddingRight = '0';
            } else {
                // Если панель скрыта, показываем её
                productInfo.style.display = 'flex';
                chatArea.style.paddingRight = '300px';
            }
        });
    }
    
    // Закрытие панели информации о товаре при клике на крестик
    if (closeProductInfoButton) {
        closeProductInfoButton.addEventListener('click', function() {
            productInfo.style.display = 'none';
            chatArea.style.paddingRight = '0';
        });
    }
    
    // Функционал для панели эмодзи
    
    // Открытие/закрытие панели эмодзи
    emojiButton.addEventListener('click', function(e) {
        e.stopPropagation(); // Предотвращаем закрытие при клике на саму кнопку
        emojiPanel.classList.toggle('active');
        emojiButton.classList.toggle('active');
    });
    
    // Закрытие панели эмодзи при клике вне неё
    document.addEventListener('click', function(e) {
        if (!emojiPanel.contains(e.target) && e.target !== emojiButton) {
            emojiPanel.classList.remove('active');
            emojiButton.classList.remove('active');
        }
    });
    
    // Переключение вкладок эмодзи
    emojiTabs.forEach(tab => {
        tab.addEventListener('click', function() {
            // Удаляем активный класс у всех вкладок
            emojiTabs.forEach(t => t.classList.remove('active'));
            
            // Добавляем активный класс текущей вкладке
            this.classList.add('active');
            
            // Получаем ID группы эмодзи для активации
            const targetGroup = this.getAttribute('data-tab');
            
            // Скрываем все группы эмодзи
            document.querySelectorAll('.emoji-group').forEach(group => {
                group.classList.remove('active');
            });
            
            // Показываем нужную группу
            document.getElementById(targetGroup).classList.add('active');
        });
    });
    
    // Вставка эмодзи в текстовое поле
    emojiItems.forEach(item => {
        item.addEventListener('click', function() {
            // Получаем текущее значение поля ввода
            const cursorPos = messageInput.selectionStart;
            const textBefore = messageInput.value.substring(0, cursorPos);
            const textAfter = messageInput.value.substring(cursorPos);
            
            // Вставляем эмодзи в позицию курсора
            messageInput.value = textBefore + this.innerText + textAfter;
            
            // Восстанавливаем фокус и позицию курсора после эмодзи
            messageInput.focus();
            messageInput.selectionStart = cursorPos + this.innerText.length;
            messageInput.selectionEnd = cursorPos + this.innerText.length;
        });
    });
    
    // Функция загрузки истории сообщений
    function loadChatHistory(chatUserId) {
        // Очищаем контейнер сообщений
        messagesContainer.innerHTML = `
            <div class="loading-messages">
                <div class="spinner"></div>
                <p>Загрузка сообщений...</p>
            </div>
        `;
        
        // Загружаем историю сообщений
        fetch(`/api/messages?userId=${userId}&chatWith=${chatUserId}`)
            .then(response => {
                if (!response.ok) {
                    throw new Error(`HTTP ошибка: ${response.status}`);
                }
                return response.json();
            })
            .then(data => {
                console.log('Получена история сообщений:', data);
                
                // Очищаем контейнер от индикатора загрузки
                messagesContainer.innerHTML = '';
                
                // Проверяем, есть ли сообщения
                if (data.messages && data.messages.length > 0) {
                    // Сортируем сообщения по дате (от старых к новым)
                    const sortedMessages = data.messages.sort((a, b) => {
                        const dateA = new Date(a.createdAt || a.timestamp);
                        const dateB = new Date(b.createdAt || b.timestamp);
                        return dateA - dateB;
                    });
                    
                    let currentDate = null;
                    
                    // Отображаем сообщения из истории
                    sortedMessages.forEach(msg => {
                        const messageDate = new Date(msg.createdAt || msg.timestamp);
                        const formattedDate = formatMessageDate(messageDate);
                        
                        // Если дата изменилась, добавляем разделитель
                        if (currentDate !== formattedDate.date) {
                            currentDate = formattedDate.date;
                            const dateDivider = document.createElement('div');
                            dateDivider.classList.add('date-divider');
                            dateDivider.innerHTML = `<span>${currentDate}</span>`;
                            messagesContainer.appendChild(dateDivider);
                        }
                        
                        const isIncoming = msg.fromId !== userId;
                        displayMessage(msg.content, isIncoming, formattedDate.time);
                    });
                    
                    // Прокручиваем к последнему сообщению
                    setTimeout(() => {
                        messagesContainer.scrollTop = messagesContainer.scrollHeight;
                    }, 100);
                } else {
                    // Если сообщений нет, показываем соответствующее уведомление
                    messagesContainer.innerHTML = `
                        <div class="no-messages">
                            <p>У вас пока нет сообщений с этим пользователем</p>
                            <p>Начните общение прямо сейчас!</p>
                        </div>
                    `;
                }
            })
            .catch(error => {
                console.error('Ошибка при загрузке истории сообщений:', error);
                
                // В случае ошибки показываем сообщение об ошибке
                messagesContainer.innerHTML = `
                    <div class="error-message">
                        <p>Не удалось загрузить историю сообщений</p>
                        <p>Пожалуйста, попробуйте позже или обновите страницу</p>
                    </div>
                `;
            });
    }
    
    // Функция для отображения сообщения в чате
    function displayMessage(text, isIncoming, timestamp) {
        // Форматируем время
        if (!timestamp) {
            const now = new Date();
            timestamp = `${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}`;
        }
        
        // Создаем элемент сообщения
        const messageElement = document.createElement('div');
        messageElement.classList.add('message', isIncoming ? 'incoming' : 'outgoing');
        
        if (isIncoming) {
            // Аватар для входящего сообщения
            const avatarSrc = document.querySelector('.chat-header .avatar img').src;
            
            messageElement.innerHTML = `
                <div class="avatar">
                    <img src="${avatarSrc}" alt="Аватар">
                </div>
                <div class="message-content">
                    <div class="message-bubble">
                        <p>${text}</p>
                    </div>
                    <div class="message-time">${timestamp}</div>
                </div>
            `;
        } else {
            messageElement.innerHTML = `
                <div class="message-content">
                    <div class="message-bubble">
                        <p>${text}</p>
                    </div>
                    <div class="message-time">${timestamp}</div>
                </div>
            `;
        }
        
        // Добавляем сообщение в контейнер
        messagesContainer.appendChild(messageElement);
        
        // Применяем уникальную анимацию к последнему сообщению
        setTimeout(() => {
            messageElement.style.animationDelay = '0s';
        }, 10);
        
        // Прокручиваем к новому сообщению
        messagesContainer.scrollTop = messagesContainer.scrollHeight;
    }
    
    // Функция отправки сообщения
    function sendMessage() {
        const messageText = messageInput.value.trim();
        
        if (messageText) {
            // Отправляем сообщение на сервер
            if (sendMessageToServer(messageText)) {
                // Текущее время
                const now = new Date();
                const formattedDate = formatMessageDate(now);
                
                // Проверяем, нужно ли добавить разделитель даты
                const lastDivider = messagesContainer.querySelector('.date-divider:last-of-type');
                const lastDividerText = lastDivider ? lastDivider.querySelector('span').textContent : null;
                
                if (!lastDividerText || lastDividerText !== formattedDate.date) {
                    const dateDivider = document.createElement('div');
                    dateDivider.classList.add('date-divider');
                    dateDivider.innerHTML = `<span>${formattedDate.date}</span>`;
                    messagesContainer.appendChild(dateDivider);
                }
                
                // Отображаем сообщение в чате
                displayMessage(messageText, false, formattedDate.time);
                
                // Обновляем последнее сообщение в списке чатов
                updateLastMessage(currentChatUserId, messageText);
                
                // Очищаем поле ввода
                messageInput.value = '';
                
                // Закрываем панель эмодзи, если она открыта
                emojiPanel.classList.remove('active');
                emojiButton.classList.remove('active');
            }
        }
    }
    
    // Функция для показа индикатора печатания
    function showTypingIndicator() {
        const typingIndicator = document.querySelector('.typing-indicator');
        
        // Если индикатор уже существует, активируем его
        if (typingIndicator) {
            typingIndicator.classList.add('active');
        } else {
            // Создаем элемент индикатора печатания
            const avatarSrc = document.querySelector('.chat-header .avatar img').src;
            const newTypingIndicator = document.createElement('div');
            newTypingIndicator.classList.add('typing-indicator');
            
            newTypingIndicator.innerHTML = `
                <div class="avatar">
                    <img src="${avatarSrc}" alt="Аватар">
                </div>
                <div class="typing-bubble">
                    <span class="typing-text">печатает</span>
                    <div class="typing-dots">
                        <span class="typing-dot"></span>
                        <span class="typing-dot"></span>
                        <span class="typing-dot"></span>
                    </div>
                </div>
            `;
            
            // Добавляем индикатор в контейнер
            messagesContainer.appendChild(newTypingIndicator);
            
            // Активируем индикатор с небольшой задержкой
            setTimeout(() => {
                newTypingIndicator.classList.add('active');
            }, 10);
            
            // Прокручиваем к индикатору
            messagesContainer.scrollTop = messagesContainer.scrollHeight;
        }
    }
    
    // Функция для скрытия индикатора печатания
    function hideTypingIndicator() {
        const typingIndicator = document.querySelector('.typing-indicator');
        
        if (typingIndicator) {
            typingIndicator.classList.remove('active');
            
            // Удаляем индикатор после завершения анимации
            setTimeout(() => {
                if (typingIndicator.parentNode) {
                    typingIndicator.parentNode.removeChild(typingIndicator);
                }
            }, 300);
        }
    }
    
    // Функция обновления шапки чата при выборе пользователя
    function updateChatHeader(chatItem) {
        // Проверяем, что chatItem существует
        if (!chatItem) {
            console.warn('Не удалось обновить заголовок чата: chatItem is null');
            
            // Устанавливаем заголовок по умолчанию
            const titleElement = document.querySelector('.chat-header .user-info h3');
            if (titleElement) {
                titleElement.textContent = 'Выберите чат';
            }
            
            return;
        }

        try {
            const userNameElement = chatItem.querySelector('.chat-item-name');
            const userName = userNameElement ? userNameElement.textContent : 'Выберите чат';
            
            const avatarImg = chatItem.querySelector('.chat-item-avatar img');
            const avatarSrc = avatarImg ? avatarImg.src : '/images/avatar.png';
            
            // Обновляем заголовок, если элемент существует
            const titleElement = document.querySelector('.chat-header .user-info h3');
            if (titleElement) {
                titleElement.textContent = userName;
            }
            
            // Обновляем аватар, если элемент существует
            const headerAvatar = document.querySelector('.chat-header .avatar img');
            if (headerAvatar) {
                headerAvatar.src = avatarSrc;
            }
        } catch (error) {
            console.error('Ошибка при обновлении шапки чата:', error);
            
            // Устанавливаем заголовок по умолчанию в случае ошибки
            const titleElement = document.querySelector('.chat-header .user-info h3');
            if (titleElement) {
                titleElement.textContent = 'Выберите чат';
            }
        }
    }
    
    // Прокручиваем чат вниз при загрузке
    messagesContainer.scrollTop = messagesContainer.scrollHeight;
    
    // Функция для показа формы создания нового чата
    function showNewChatForm() {
        const newChatOverlay = document.getElementById('newChatOverlay');
        if (newChatOverlay) {
            newChatOverlay.style.display = 'flex';
        }
    }
    
    // Функция для скрытия формы создания нового чата
    function hideNewChatForm() {
        const newChatOverlay = document.getElementById('newChatOverlay');
        if (newChatOverlay) {
            newChatOverlay.style.display = 'none';
        }
    }
    
    // Функция создания нового чата
    function createNewChat() {
        const recipientId = parseInt(document.getElementById('recipientIdInput').value);
        const productId = parseInt(document.getElementById('productIdInput').value);
        const initialMessage = document.getElementById('initialMessageInput').value.trim();
        
        if (!recipientId || !productId || !initialMessage) {
            alert('Пожалуйста, заполните все поля формы');
            return;
        }
        
        // Отправляем первое сообщение через WebSocket
        if (socket && socket.readyState === WebSocket.OPEN) {
            const message = {
                type: 'message',
                fromId: userId,
                toId: recipientId,
                productId: productId,
                content: initialMessage
            };
            
            socket.send(JSON.stringify(message));
            
            // Скрываем форму
            hideNewChatForm();
            
            // Очищаем форму
            document.getElementById('initialMessageInput').value = '';
            
            // Перезагружаем список чатов через 1 секунду (даем время для создания чата на сервере)
            setTimeout(() => {
                loadUserChats();
            }, 1000);
            
            return true;
        } else {
            alert("WebSocket соединение не установлено. Пожалуйста, попробуйте позже.");
            return false;
        }
    }
    
    // Обработчики кнопок формы нового чата
    const newChatBtn = document.getElementById('newChatBtn');
    if (newChatBtn) {
        newChatBtn.addEventListener('click', showNewChatForm);
    }
    
    const cancelNewChatButton = document.getElementById('cancelNewChatButton');
    if (cancelNewChatButton) {
        cancelNewChatButton.addEventListener('click', hideNewChatForm);
    }
    
    const startNewChatButton = document.getElementById('startNewChatButton');
    if (startNewChatButton) {
        startNewChatButton.addEventListener('click', createNewChat);
    }

    // Функция для форматирования даты сообщения
    function formatMessageDate(date) {
        const now = new Date();
        const messageDate = new Date(date);
        
        // Форматируем время
        const hours = messageDate.getHours().toString().padStart(2, '0');
        const minutes = messageDate.getMinutes().toString().padStart(2, '0');
        const timeStr = `${hours}:${minutes}`;
        
        // Если сообщение сегодня
        if (messageDate.toDateString() === now.toDateString()) {
            return { date: 'Сегодня', time: timeStr };
        }
        
        // Если сообщение вчера
        const yesterday = new Date(now);
        yesterday.setDate(yesterday.getDate() - 1);
        if (messageDate.toDateString() === yesterday.toDateString()) {
            return { date: 'Вчера', time: timeStr };
        }
        
        // Если сообщение в этом году
        const months = [
            'января', 'февраля', 'марта', 'апреля', 'мая', 'июня',
            'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря'
        ];
        
        if (messageDate.getFullYear() === now.getFullYear()) {
            return {
                date: `${messageDate.getDate()} ${months[messageDate.getMonth()]}`,
                time: timeStr
            };
        }
        
        // Если сообщение в другом году
        return {
            date: `${messageDate.getDate()} ${months[messageDate.getMonth()]} ${messageDate.getFullYear()}`,
            time: timeStr
        };
    }

    // Добавляем функцию обновления статуса пользователя
    function updateUserStatus(userId, status) {
        console.log(`📊 Обновление статуса пользователя ${userId}: ${status}`);
        const chatItem = document.querySelector(`.chat-item[data-userId="${userId}"]`);
        if (chatItem) {
            const statusElement = chatItem.querySelector('.chat-item-status');
            if (statusElement) {
                // Удаляем все классы статусов
                statusElement.classList.remove('online', 'offline', 'away');
                // Добавляем новый класс статуса
                statusElement.classList.add(status);
            }
        }
    }

    // Обработка закрытия страницы/вкладки
    function handlePageUnload() {
        if (socket && socket.readyState === WebSocket.OPEN) {
            isIntentionalClose = true;
            const statusMsg = {
                type: 'status',
                userId: userId,
                status: 'offline'
            };
            
            // Используем синхронный XMLHttpRequest для гарантированной отправки статуса
            const xhr = new XMLHttpRequest();
            xhr.open('POST', '/api/status', false); // false = синхронный запрос
            xhr.setRequestHeader('Content-Type', 'application/json');
            xhr.send(JSON.stringify(statusMsg));
            
            // Закрываем WebSocket соединение
            socket.close();
        }
    }

    // Добавляем обработчики закрытия страницы
    window.addEventListener('beforeunload', handlePageUnload);
    window.addEventListener('unload', handlePageUnload);
    window.addEventListener('visibilitychange', function() {
        if (document.visibilityState === 'hidden') {
            handlePageUnload();
        } else if (document.visibilityState === 'visible' && (!socket || socket.readyState !== WebSocket.OPEN)) {
            // Переподключаемся при возвращении на страницу
            reconnectAttempts = 0;
            initWebSocket();
        }
    });

    // Функция выбора чата
    function selectChat(otherUserId) {
        console.log(`Выбран чат с пользователем ${otherUserId}`);
        
        if (!otherUserId) {
            console.warn('Не удалось выбрать чат: otherUserId is null');
            return;
        }
        
        try {
            // Обновляем текущего собеседника
            currentChatUserId = otherUserId;
            
            // Находим элемент чата
            const chatItem = findChatItemByUserId(otherUserId);
            
            if (chatItem) {
                // Удаляем активный класс у всех элементов
                document.querySelectorAll('.chat-item').forEach(item => item.classList.remove('active'));
                
                // Добавляем активный класс выбранному элементу
                chatItem.classList.add('active');
                
                // Обновляем шапку чата
                updateChatHeader(chatItem);
                
                // На мобильном: показываем область чата, скрываем список чатов
                if (isMobile) {
                    const chatList = document.querySelector('.chat-list');
                    const chatArea = document.querySelector('.chat-area');
                    
                    if (chatList) chatList.classList.remove('active');
                    if (chatArea) {
                        chatArea.classList.add('active');
                        chatArea.classList.remove('hidden');
                    }
                }
                
                // Загружаем историю сообщений
                loadChatHistory(otherUserId);
            } else {
                console.warn(`Не найден элемент чата для пользователя с ID ${otherUserId}`);
                // Обновляем шапку чата с заголовком по умолчанию
                updateChatHeader(null);
            }
        } catch (error) {
            console.error('Ошибка при выборе чата:', error);
        }
    }
}); 