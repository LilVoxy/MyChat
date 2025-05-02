#!/bin/bash

# Скрипт для запуска линейной регрессии как отдельного процесса
# Автор: LilVoxy
# Дата: 2025-05-02

# Определение переменных с параметрами по умолчанию
DAYS=30              # Количество дней для анализа
FORECAST=14          # Количество дней для прогноза
CONFIDENCE=0.95      # Уровень доверия (0.90, 0.95, 0.99)
MIN_R2=0.30          # Минимальный порог для R²

# Цвета для вывода
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Функция для вывода справки
function show_help {
    echo "Утилита для запуска линейной регрессии на данных активности пользователей"
    echo
    echo "Использование:"
    echo "  ./run_lr.sh [опции]"
    echo
    echo "Опции:"
    echo "  -d, --days ЧИСЛО      Количество дней для анализа (по умолчанию: $DAYS)"
    echo "  -f, --forecast ЧИСЛО  Количество дней для прогноза (по умолчанию: $FORECAST)"
    echo "  -c, --confidence ЧИСЛО Уровень доверия (0.90, 0.95, 0.99) (по умолчанию: $CONFIDENCE)"
    echo "  -r, --min-r2 ЧИСЛО    Минимальный порог для R² (по умолчанию: $MIN_R2)"
    echo "  -h, --help            Показать эту справку"
    echo
}

# Разбор параметров командной строки
while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
        -d|--days)
            DAYS="$2"
            shift 2
            ;;
        -f|--forecast)
            FORECAST="$2"
            shift 2
            ;;
        -c|--confidence)
            CONFIDENCE="$2"
            shift 2
            ;;
        -r|--min-r2)
            MIN_R2="$2"
            shift 2
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo -e "${RED}Неизвестный параметр: $1${NC}"
            show_help
            exit 1
            ;;
    esac
done

# Проверка, установлен ли Go
if ! command -v go &> /dev/null; then
    echo -e "${RED}Ошибка: Go не установлен. Пожалуйста, установите Go перед запуском этого скрипта.${NC}"
    exit 1
fi

echo -e "${GREEN}Запуск линейной регрессии со следующими параметрами:${NC}"
echo -e "  Дни для анализа:  ${YELLOW}$DAYS${NC}"
echo -e "  Дни для прогноза: ${YELLOW}$FORECAST${NC}"
echo -e "  Уровень доверия:  ${YELLOW}$CONFIDENCE${NC}"
echo -e "  Порог R²:         ${YELLOW}$MIN_R2${NC}"
echo

# Запуск Go-скрипта с передачей параметров
echo -e "${GREEN}Выполнение...${NC}"
cd $(dirname $0)/..
GO_CMD="go run etl_runner.go -mode=lr -days=$DAYS -forecast=$FORECAST -confidence=$CONFIDENCE -min-r2=$MIN_R2"
echo $GO_CMD
$GO_CMD

# Проверка статуса выполнения
STATUS=$?
if [ $STATUS -eq 0 ]; then
    echo -e "${GREEN}Линейная регрессия успешно выполнена!${NC}"
else
    echo -e "${RED}Ошибка при выполнении линейной регрессии. Код ошибки: $STATUS${NC}"
fi

exit $STATUS 