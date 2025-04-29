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
    hour_of_day TINYINT,              -- час суток (0-23)
    UNIQUE KEY (full_date, hour_of_day)
);
-- Измерение пользователей user_dimension)
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
-- Факты почасовой активности (hourly_activity_facts)
CREATE TABLE chat_analytics.hourly_activity_facts (
    id INT AUTO_INCREMENT PRIMARY KEY,
    date_id INT,                       -- связь с датой из временного измерения
    hour_of_day TINYINT,               -- час дня (0-23)
    total_messages INT,                -- сообщений за час
    total_new_chats INT,               -- новых чатов за час
    active_users INT,                  -- активных пользователей за час
    avg_response_time_minutes FLOAT,   -- среднее время ответа в минутах
    UNIQUE KEY (date_id, hour_of_day),
    FOREIGN KEY (date_id) REFERENCES time_dimension(id)
);