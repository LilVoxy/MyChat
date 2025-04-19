// Конфигурация приложения
const CONFIG = {
    // URL API
    API_URL: window.location.origin,
    
    // WebSocket URL
    WS_URL: `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/ws`,
    
    // Интервал обновления статусов пользователей
    STATUS_INTERVAL: 15000, // 15 секунд
    
    // Время задержки перед автоматической прокруткой чата
    SCROLL_DELAY: 100,
    
    // Время последнего активного статуса (в миллисекундах)
    ONLINE_THRESHOLD: 60000, // 1 минута
    
    // Заполнители для отсутствующих изображений
    DEFAULT_AVATAR: 'images/default-avatar.png',
    DEFAULT_PRODUCT_IMAGE: 'images/default-product.png',
    
    // ID текущего пользователя (должен быть установлен после авторизации)
    // В реальном приложении это должно быть получено из механизма авторизации
    CURRENT_USER_ID: 1 // Временное решение для демонстрации
};

// Режим отладки
const DEBUG = true;

// Функция для вывода отладочных сообщений
function log(...args) {
    if (DEBUG) {
        console.log(...args);
    }
} 