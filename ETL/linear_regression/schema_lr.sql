-- Создание таблицы для хранения результатов линейной регрессии
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