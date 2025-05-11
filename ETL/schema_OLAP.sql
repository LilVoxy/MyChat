-- Таблицы измерений
-- Временное измерение (time_dimension)
CREATE TABLE chat_analytics.time_dimension (
    id INT AUTO_INCREMENT PRIMARY KEY,
    full_date DATE,                   -- полная дата (например, 2023-05-15)
    year SMALLINT,                    -- год (например, 2023)
    quarter TINYINT,                  -- квартал (1-4)
    month TINYINT,                    -- месяц (1-12)
    month_name VARCHAR(10),           -- название месяца (January, February...)
    week_of_year TINYINT,             -- неделя года (1-53)
    day_of_month TINYINT,             -- день месяца (1-31)
    day_of_week TINYINT,              -- день недели (1-7, где 1=воскресенье)
    day_name VARCHAR(10),             -- название дня (Monday, Tuesday...)
    is_weekend BOOLEAN,               -- признак выходного дня (TRUE/FALSE)
    UNIQUE KEY (full_date, hour_of_day)
);
-- Измерение пользователей user_dimension
CREATE TABLE chat_analytics.user_dimension (
    id INT PRIMARY KEY,                         -- ID пользователя из основной БД
    registration_date DATE,                     -- дата регистрации
    days_active INT,                            -- дней с момента регистрации
    total_chats INT,                            -- общее количество чатов
    total_messages INT,                         -- общее количество сообщений
    avg_response_time_minutes FLOAT,            -- среднее время ответа в минутах
    activity_level ENUM('high', 'medium', 'low'), -- уровень активности
    last_updated TIMESTAMP                      -- время последнего обновления
);
-- Таблицы фактов
-- Факты сообщений (message_facts)
CREATE TABLE chat_analytics.message_facts (
    id INT AUTO_INCREMENT PRIMARY KEY,
    message_id INT UNIQUE,              -- ID сообщения из исходной базы
    time_id INT,                         -- связь с временным измерением
    sender_id INT,                       -- ID отправителя
    recipient_id INT,                    -- ID получателя
    chat_id INT,                         -- ID чата
    message_length INT,                  -- длина сообщения в символах
    response_time_minutes FLOAT,         -- время ответа в минутах
    is_first_in_chat BOOLEAN,            -- признак первого сообщения в чате
    FOREIGN KEY (time_id) REFERENCES time_dimension(id)
);
-- Факты чатов (chat_facts)
CREATE TABLE chat_analytics.chat_facts (
    id INT AUTO_INCREMENT PRIMARY KEY,
    chat_id INT UNIQUE,                 -- ID чата из исходной базы
    start_time_id INT,                  -- ID времени начала чата
    end_time_id INT,                    -- ID времени последнего сообщения
    buyer_id INT,                       -- ID покупателя
    seller_id INT,                      -- ID продавца
    total_messages INT,                 -- общее количество сообщений
    buyer_messages INT,                 -- сообщения от покупателя
    seller_messages INT,                -- сообщения от продавца
    avg_message_length FLOAT,           -- средняя длина сообщения
    avg_response_time_minutes FLOAT,    -- среднее время ответа в минутах
    chat_duration_hours FLOAT,          -- продолжительность чата в часах
    FOREIGN KEY (start_time_id) REFERENCES time_dimension(id),
    FOREIGN KEY (end_time_id) REFERENCES time_dimension(id)
);
-- Факты ежедневной активности (daily_activity_facts)
CREATE TABLE chat_analytics.daily_activity_facts (
    id INT AUTO_INCREMENT PRIMARY KEY,
    date_id INT,                        -- связь с временным измерением
    total_messages INT,                 -- общее количество сообщений за день
    total_new_chats INT,                -- новые чаты за день
    active_users INT,                   -- активные пользователи за день
    new_users INT,                      -- новые пользователи за день
    avg_messages_per_chat FLOAT,        -- среднее число сообщений в чате
    avg_response_time_minutes FLOAT,    -- среднее время ответа в минутах
    peak_hour TINYINT,                  -- час пиковой активности
    peak_hour_messages INT,             -- сообщений в пиковый час
    FOREIGN KEY (date_id) REFERENCES time_dimension(id)
);

-- ---------------------------------------------------------------------------------------------------------------------------------------

-- Таблица для хранения результатов линейной регрессии
CREATE TABLE IF NOT EXISTS chat_analytics.activity_trend_predictions (
    id INT AUTO_INCREMENT PRIMARY KEY,
    period_start DATE NOT NULL,                    -- Дата начала анализируемого периода
    period_end DATE NOT NULL,                      -- Дата конца анализируемого периода
    a DOUBLE NOT NULL,                             -- Коэффициент наклона (slope)
    b DOUBLE NOT NULL,                             -- Сдвиг (intercept)
    r DOUBLE NOT NULL,                             -- Коэффициент корреляции Пирсона
    r2 DOUBLE NOT NULL,                            -- Коэффициент детерминации (R^2)
    forecast_date DATE NOT NULL,                   -- Дата, на которую сделан прогноз
    forecast_value DOUBLE NOT NULL,                -- Прогнозируемое значение
    ci_lower DOUBLE NOT NULL,                      -- Нижняя граница доверительного интервала
    ci_upper DOUBLE NOT NULL,                      -- Верхняя граница доверительного интервала
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, -- Дата создания прогноза
    INDEX idx_forecast_date (forecast_date),
    INDEX idx_period (period_start, period_end)
);

-- ChatRank: Модификация алгоритма PageRank для ранжирования влиятельности пользователей в чатах
-- Таблица рангов пользователей
CREATE TABLE chat_analytics.user_influence_rank (
    id INT AUTO_INCREMENT PRIMARY KEY,                -- Уникальный идентификатор записи
    user_id INT NOT NULL,                             -- Внешний ключ на пользователя (user_dimension)
    chat_rank DOUBLE NOT NULL,                        -- Итоговый ранг пользователя (значение ChatRank)
    rank_percentile DOUBLE NOT NULL,                  -- Процентиль ранга (например, 0.95 для топ-5%)
    category ENUM('high', 'medium', 'low') NOT NULL,  -- Категория влияния (по порогам процентилей)
    calculation_date DATE NOT NULL,                   -- Дата расчёта (для историчности)
    iteration_count INT NOT NULL,                     -- Количество итераций до сходимости
    convergence_delta DOUBLE NOT NULL,                -- Дельта сходимости (максимальное изменение ранга на последней итерации)
    FOREIGN KEY (user_id) REFERENCES user_dimension(id), -- Связь с таблицей пользователей
    UNIQUE KEY (user_id),                             -- Уникальный ключ по ID пользователя для обновления
    INDEX idx_date (calculation_date),                -- Индекс для поиска по дате
    INDEX idx_rank (chat_rank)                        -- Индекс для сортировки/поиска по рангу
);

-- Таблица весов коммуникационных связей
CREATE TABLE chat_analytics.communication_weights (
    id INT AUTO_INCREMENT PRIMARY KEY,                -- Уникальный идентификатор записи
    sender_id INT NOT NULL,                           -- Внешний ключ на отправителя (user_dimension)
    recipient_id INT NOT NULL,                        -- Внешний ключ на получателя (user_dimension)
    weight DOUBLE NOT NULL,                           -- Итоговый вес связи (агрегированный)
    time_factor DOUBLE NOT NULL,                      -- Временной фактор (скорость ответа)
    response_factor DOUBLE NOT NULL,                  -- Фактор частоты ответов
    length_factor DOUBLE NOT NULL,                    -- Фактор длины сообщений
    continuation_factor DOUBLE NOT NULL,              -- Фактор количества сообщений
    calculation_date DATE NOT NULL,                   -- Дата расчёта (для историчности)
    FOREIGN KEY (sender_id) REFERENCES user_dimension(id),   -- Связь с отправителем
    FOREIGN KEY (recipient_id) REFERENCES user_dimension(id),-- Связь с получателем
    UNIQUE KEY (sender_id, recipient_id, calculation_date),  -- Гарантия уникальности пары на дату
    INDEX idx_sender (sender_id),                     -- Индекс для поиска по отправителю
    INDEX idx_recipient (recipient_id),               -- Индекс для поиска по получателю
    INDEX idx_date (calculation_date)                 -- Индекс для поиска по дате
);

-- Таблица журнала выполнения ETL процесса
CREATE TABLE IF NOT EXISTS chat_analytics.etl_run_log (
    id INT AUTO_INCREMENT PRIMARY KEY,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NULL,
    status ENUM('success', 'failed', 'in_progress') NOT NULL DEFAULT 'in_progress',
    users_processed INT DEFAULT 0,
    chats_processed INT DEFAULT 0,
    messages_processed INT DEFAULT 0,
    last_processed_message_id INT DEFAULT 0,
    error_message TEXT,
    execution_time_seconds FLOAT
);