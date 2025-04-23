document.addEventListener('DOMContentLoaded', function() {
    // Константы
    const API_URL = 'http://localhost:8080'; // Базовый URL API
    const CURRENT_USER_ID = localStorage.getItem('currentUserId') || 1; // ID текущего пользователя
    
    // Получаем ID товара из URL параметров (например: ?productId=123)
    const urlParams = new URLSearchParams(window.location.search);
    const productId = urlParams.get('productId') || 1; // По умолчанию 1, если не указан
    
    // DOM-элементы
    const productNameEl = document.getElementById('product-name');
    const productPriceEl = document.getElementById('product-price');
    const productCategoryEl = document.getElementById('product-category');
    const productDescriptionEl = document.getElementById('product-description');
    const productImageEl = document.getElementById('product-image');
    
    const sellerAvatarEl = document.getElementById('seller-avatar');
    const sellerNameEl = document.getElementById('seller-name');
    const sellerStatusEl = document.getElementById('seller-status');
    const sellerOnlineStatusEl = document.getElementById('seller-online-status');
    
    const chatButtonEl = document.getElementById('chat-button');
    const sellerPageButtonEl = document.getElementById('seller-page-button');
    const favoriteButtonEl = document.getElementById('favorite-button');
    
    // Загрузка данных о товаре
    function loadProductData() {
        // В реальном приложении здесь должен быть запрос к API
        // Для демонстрации используем заглушку
        const demoProduct = {
            id: productId,
            name: 'Комплект одежды IMPL STORE',
            price: '5 900 ₽',
            category: 'Одежда',
            description: 'Стильный молодежный комплект одежды от IMPL STORE. В набор входят джинсы с рисунком, толстовка с капюшоном и кроссовки. Отличное качество материалов, современный дизайн. Доставка по всей России.',
            image: '../images/product-demo.svg',
            seller: {
                id: 2,
                name: 'IMPL STORE',
                avatar: '../images/seller-avatar.svg',
                isOnline: true
            }
        };
        
        // Заполняем элементы страницы данными
        productNameEl.textContent = demoProduct.name;
        productPriceEl.textContent = demoProduct.price;
        productCategoryEl.textContent = demoProduct.category;
        productDescriptionEl.textContent = demoProduct.description;
        
        // Проверяем существование изображения и устанавливаем его
        const img = new Image();
        img.onload = function() {
            productImageEl.src = demoProduct.image;
        };
        img.onerror = function() {
            // Если изображение не загрузилось, оставляем плейсхолдер
            productImageEl.src = '../images/placeholder.svg';
        };
        img.src = demoProduct.image;
        
        // Данные о продавце
        sellerNameEl.textContent = demoProduct.seller.name;
        
        // Проверяем существование аватара и устанавливаем его
        const sellerImg = new Image();
        sellerImg.onload = function() {
            sellerAvatarEl.src = demoProduct.seller.avatar;
        };
        sellerImg.onerror = function() {
            // Если аватар не загрузился, используем плейсхолдер
            sellerAvatarEl.src = '../images/user-placeholder.svg';
        };
        sellerImg.src = demoProduct.seller.avatar;
        
        // Устанавливаем статус продавца
        if (demoProduct.seller.isOnline) {
            sellerStatusEl.classList.remove('offline');
            sellerStatusEl.classList.add('online');
            sellerOnlineStatusEl.textContent = 'в сети';
        } else {
            sellerStatusEl.classList.remove('online');
            sellerStatusEl.classList.add('offline');
            sellerOnlineStatusEl.textContent = 'не в сети';
        }
        
        // Сохраняем ID продавца для кнопок
        chatButtonEl.dataset.sellerId = demoProduct.seller.id;
        sellerPageButtonEl.dataset.sellerId = demoProduct.seller.id;
    }
    
    // Обработчики событий для кнопок
    
    // Кнопка "Написать" - открывает чат с продавцом по этому товару
    chatButtonEl.addEventListener('click', function() {
        const sellerId = this.dataset.sellerId;
        
        // Перенаправляем на страницу чата с нужными параметрами
        window.location.href = `../index.html?userId=${CURRENT_USER_ID}&chatWith=${sellerId}&productId=${productId}`;
    });
    
    // Кнопка "Страница продавца" - переход на страницу продавца
    sellerPageButtonEl.addEventListener('click', function() {
        const sellerId = this.dataset.sellerId;
        alert('Переход на страницу продавца (ID: ' + sellerId + ')');
        // В реальном приложении будет переход на страницу продавца
        // window.location.href = `/seller.html?sellerId=${sellerId}`;
    });
    
    // Кнопка "В избранное" - добавляет/удаляет товар из избранного
    favoriteButtonEl.addEventListener('click', function() {
        // Переключаем класс активности
        this.classList.toggle('active');
        
        // Меняем иконку
        const icon = this.querySelector('i');
        if (this.classList.contains('active')) {
            icon.classList.remove('far');
            icon.classList.add('fas');
            alert('Товар добавлен в избранное');
        } else {
            icon.classList.remove('fas');
            icon.classList.add('far');
            alert('Товар удален из избранного');
        }
        
        // В реальном приложении здесь будет запрос к API для сохранения/удаления из избранного
    });
    
    // Загружаем данные при загрузке страницы
    loadProductData();
}); 