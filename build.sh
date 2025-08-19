#!/bin/bash
# Останавливаем выполнение скрипта, если любая команда завершится с ошибкой
set -e

CONFIG_FILE="config.json"

# --- Проверка наличия jq и config.json ---
if ! command -v jq &> /dev/null; then
    echo "ОШИБКА: Утилита 'jq' не найдена. Пожалуйста, установите ее."
    echo "Ubuntu/Debian: sudo apt-get install jq"
    echo "macOS: brew install jq"
    exit 1
fi

if [ ! -f "$CONFIG_FILE" ]; then
    echo "ОШИБКА: Файл конфигурации '$CONFIG_FILE' не найден."
    echo "Сначала запустите 'go run get_ids_config.go'."
    exit 1
fi

echo "--- Читаем конфигурацию из $CONFIG_FILE... ---"
# --- Читаем сырые данные из JSON ---
BOT_TOKEN=$(jq -r '.bot_token // ""' $CONFIG_FILE)
SCREENSHOT_TARGET=$(jq -r '.screenshot_target // ""' $CONFIG_FILE)
LOG_TARGET=$(jq -r '.log_target // ""' $CONFIG_FILE)
NAVY_KEYS=$(jq -r '.api_keys.navy_ai | join(",")' $CONFIG_FILE)
ELECTRON_KEYS=$(jq -r '.api_keys.electron_hub | join(",")' $CONFIG_FILE)
VOID_KEYS=$(jq -r '.api_keys.void_ai | join(",")' $CONFIG_FILE)
OCR_KEYS=$(jq -r '.api_keys.ocr_space | join(",")' $CONFIG_FILE)

# Проверяем, что основные поля не пустые
if [ -z "$BOT_TOKEN" ] || [ -z "$SCREENSHOT_TARGET" ]; then
    echo "ОШИБКА: Поля 'bot_token' и 'screenshot_target' в $CONFIG_FILE не могут быть пустыми."
    echo "Запустите 'go run get_ids_config.go', чтобы их заполнить."
    exit 1
fi

echo "--- Шифруем секреты... ---"

# --- ИСПРАВЛЕНО: Передаем переменные как аргументы, а не через pipe ---
ENC_TOKEN=$(go run . -encrypt "$BOT_TOKEN")
ENC_SCREENSHOT_TARGET=$(go run . -encrypt "$SCREENSHOT_TARGET")
ENC_LOG_TARGET=$(go run . -encrypt "$LOG_TARGET")
ENC_NAVY_KEYS=$(go run . -encrypt "$NAVY_KEYS")
ENC_ELECTRON_KEYS=$(go run . -encrypt "$ELECTRON_KEYS")
ENC_VOID_KEYS=$(go run . -encrypt "$VOID_KEYS")
ENC_OCR_KEYS=$(go run . -encrypt "$OCR_KEYS")

echo "Секреты успешно зашифрованы."
echo "--- Компилируем приложение для Windows (amd64)... ---"

# --- Собираем финальное приложение с внедрением зашифрованных переменных ---
GOOS=windows GOARCH=amd64 go build -o winscreensender.exe -ldflags=" \
    -X 'main.encryptedToken=$ENC_TOKEN' \
    -X 'main.encryptedScreenshotTargetInfo=$ENC_SCREENSHOT_TARGET' \
    -X 'main.encryptedLogTargetInfo=$ENC_LOG_TARGET' \
    -X 'main.encryptedNavyAIKeys=$ENC_NAVY_KEYS' \
    -X 'main.encryptedElectronHubKeys=$ENC_ELECTRON_KEYS' \
    -X 'main.encryptedVoidAIKeys=$ENC_VOID_KEYS' \
    -X 'main.encryptedOcrKeys=$ENC_OCR_KEYS' \
    -H=windowsgui" .

echo "✅ УСПЕХ! Приложение 'winscreensender.exe' успешно скомпилировано."
