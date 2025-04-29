#!/bin/bash

# Скрипт для запуска ETL-процесса

# Переходим в директорию ETL
cd "$(dirname "$0")"

# Проверяем, установлены ли зависимости
if ! go list -m github.com/go-co-op/gocron &> /dev/null; then
    echo "Устанавливаем зависимость gocron..."
    go get github.com/go-co-op/gocron
fi

# Компилируем ETL Runner
echo "Компилируем ETL Runner..."
go build -o etl_runner etl_runner.go

# Проверка успешности компиляции
if [ $? -ne 0 ]; then
    echo "❌ Ошибка компиляции!"
    exit 1
fi

# Парсим аргументы
MODE="scheduled"
if [ "$1" == "once" ]; then
    MODE="once"
    echo "Запуск ETL в режиме однократного выполнения..."
else
    echo "Запуск ETL в режиме планировщика..."
fi

# Запускаем ETL Runner
if [ "$MODE" == "once" ]; then
    # Временно изменяем код для запуска RunOnce
    sed -i'.bak' 's/RunScheduled()/RunOnce()/g' etl_runner.go
    go build -o etl_runner etl_runner.go
    ./etl_runner
    # Возвращаем оригинальный код
    mv etl_runner.go.bak etl_runner.go
else
    ./etl_runner
fi

echo "ETL процесс завершен." 