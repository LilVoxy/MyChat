#!/bin/bash

# Скрипт для запуска ChatRank

# Переходим в корневую директорию ETL
cd "$(dirname "$0")/.."

# Определяем переменные по умолчанию
DAMPING=0.85
MAX_ITERATIONS=100
EPSILON=0.0001
TIME_FACTOR=0.25
RESPONSE_FACTOR=0.25
LENGTH_FACTOR=0.25
CONTINUATION_FACTOR=0.25

# Функция для вывода справки
show_help() {
    echo "Использование: $0 [опции]"
    echo "Опции:"
    echo "  -d, --damping ЧИСЛО            Коэффициент затухания (по умолчанию: $DAMPING)"
    echo "  -i, --iterations ЧИСЛО         Максимальное количество итераций (по умолчанию: $MAX_ITERATIONS)"
    echo "  -e, --epsilon ЧИСЛО            Порог сходимости (по умолчанию: $EPSILON)"
    echo "  -tf, --time-factor ЧИСЛО       Вес временного фактора (по умолчанию: $TIME_FACTOR)"
    echo "  -rf, --response-factor ЧИСЛО   Вес фактора частоты ответов (по умолчанию: $RESPONSE_FACTOR)"
    echo "  -lf, --length-factor ЧИСЛО     Вес фактора длины сообщений (по умолчанию: $LENGTH_FACTOR)"
    echo "  -cf, --continuation-factor ЧИСЛО Вес фактора количества сообщений (по умолчанию: $CONTINUATION_FACTOR)"
    echo "  -h, --help                     Показать эту справку"
    echo
    echo "Примеры:"
    echo "  $0"
    echo "  $0 --damping 0.9 --iterations 200"
    echo "  $0 --time-factor 0.4 --response-factor 0.3 --length-factor 0.2 --continuation-factor 0.1"
}

# Парсим аргументы командной строки
while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
        -d|--damping)
            DAMPING="$2"
            shift 2
            ;;
        -i|--iterations)
            MAX_ITERATIONS="$2"
            shift 2
            ;;
        -e|--epsilon)
            EPSILON="$2"
            shift 2
            ;;
        -tf|--time-factor)
            TIME_FACTOR="$2"
            shift 2
            ;;
        -rf|--response-factor)
            RESPONSE_FACTOR="$2"
            shift 2
            ;;
        -lf|--length-factor)
            LENGTH_FACTOR="$2"
            shift 2
            ;;
        -cf|--continuation-factor)
            CONTINUATION_FACTOR="$2"
            shift 2
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo "Неизвестная опция: $key"
            show_help
            exit 1
            ;;
    esac
done

# Проверяем, установлены ли зависимости
if ! go list -m github.com/go-co-op/gocron &> /dev/null; then
    echo "Устанавливаем зависимость gocron..."
    go get github.com/go-co-op/gocron
fi

# Проверяем сумму весов
SUM=$(echo "$TIME_FACTOR + $RESPONSE_FACTOR + $LENGTH_FACTOR + $CONTINUATION_FACTOR" | bc -l)
if (( $(echo "$SUM != 1.0" | bc -l) )); then
    echo "Предупреждение: Сумма весов факторов ($SUM) не равна 1.0"
    echo "Веса будут нормализованы автоматически"
fi

# Компилируем и запускаем
echo "Запуск ChatRank со следующими параметрами:"
echo "- Коэффициент затухания: $DAMPING"
echo "- Максимум итераций: $MAX_ITERATIONS"
echo "- Порог сходимости: $EPSILON"
echo "- Вес временного фактора: $TIME_FACTOR"
echo "- Вес фактора частоты ответов: $RESPONSE_FACTOR"
echo "- Вес фактора длины сообщений: $LENGTH_FACTOR"
echo "- Вес фактора количества сообщений: $CONTINUATION_FACTOR"

# Запускаем ETL Runner в режиме ChatRank
go build -o etl_runner etl_runner.go

./etl_runner -mode=cr -damping=$DAMPING -iterations=$MAX_ITERATIONS -epsilon=$EPSILON \
    -time-factor=$TIME_FACTOR -response-factor=$RESPONSE_FACTOR \
    -length-factor=$LENGTH_FACTOR -continuation-factor=$CONTINUATION_FACTOR

echo "ChatRank завершен" 