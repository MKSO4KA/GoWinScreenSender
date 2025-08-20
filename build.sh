#!/bin/bash
# Останавливаем выполнение скрипта, если любая команда завершится с ошибкой.
set -e

# --- Переменные ---
CONFIG_FILE="config.json"
OUTPUT_NAME="ScreenSender.exe"

# --- Шаг 1: Проверка наличия config.json ---
if [ ! -f "$CONFIG_FILE" ]; then
    echo "❌ ОШИБКА: Файл конфигурации '$CONFIG_FILE' не найден."
    echo "   Пожалуйста, создайте этот файл и заполните его вашими данными."
    exit 1
fi

echo "✅ Конфигурация найдена. Начинаю компиляцию для Windows (amd64)..."

# --- Шаг 2: Компиляция приложения ---
# GOOS=windows GOARCH=amd64 - собираем приложение для Windows
# -o "$OUTPUT_NAME" - задаем имя выходного файла
# -ldflags="-H=windowsgui" - скрывает черное консольное окно при запуске .exe
GOOS=windows GOARCH=amd64 go build -o "$OUTPUT_NAME" -ldflags="-H=windowsgui" .

echo "✅ УСПЕХ! Приложение '$OUTPUT_NAME' успешно скомпилировано."