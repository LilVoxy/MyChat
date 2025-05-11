-- Скрипт для очистки всех таблиц в OLAP-базе (chat_analytics)
-- Порядок очистки с временным отключением проверки внешних ключей

-- Временно отключаем проверку внешних ключей
SET FOREIGN_KEY_CHECKS = 0;

-- Очищаем все таблицы
TRUNCATE TABLE chat_analytics.communication_weights;
TRUNCATE TABLE chat_analytics.user_influence_rank;
TRUNCATE TABLE chat_analytics.activity_trend_predictions;
TRUNCATE TABLE chat_analytics.daily_activity_facts;
TRUNCATE TABLE chat_analytics.message_facts;
TRUNCATE TABLE chat_analytics.chat_facts;
TRUNCATE TABLE chat_analytics.user_dimension;
TRUNCATE TABLE chat_analytics.etl_run_log;

-- Включаем обратно проверку внешних ключей
SET FOREIGN_KEY_CHECKS = 1;

-- Примечание: таблица time_dimension не очищается, так как она содержит базовые данные о датах
-- Если нужно очистить и её, раскомментируйте строку ниже
-- TRUNCATE TABLE chat_analytics.time_dimension; 