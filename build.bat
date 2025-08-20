@echo off
setlocal

:: --- Переменные ---
set CONFIG_FILE=config.json
set OUTPUT_NAME=ScreenSender.exe

:: --- Шаг 1: Проверка наличия config.json ---
echo Проверка наличия файла конфигурации...
if not exist "%CONFIG_FILE%" (
    echo.
    echo ^>^>^> ОШИБКА: Файл конфигурации '%CONFIG_FILE%' не найден.
    echo ^>^>^> Пожалуйста, создайте этот файл и заполните его вашими данными.
    echo.
    exit /b 1
)

echo Конфигурация найдена. Начинаю компиляцию...
echo.

:: --- Шаг 2: Компиляция приложения ---
:: Устанавливаем переменные окружения для сборки под Windows
set GOOS=windows
set GOARCH=amd64

:: Запускаем сборку
:: -o "%OUTPUT_NAME%" - задаем имя выходного файла
:: -ldflags="-H=windowsgui" - скрывает черное консольное окно при запуске .exe
go build -o "%OUTPUT_NAME%" -ldflags="-H=windowsgui" .

:: --- Шаг 3: Проверка результата ---
if %ERRORLEVEL% neq 0 (
    echo.
    echo ^<.^<.^< КРИТИЧЕСКАЯ ОШИБКА: Сборка не удалась. Проверьте сообщения об ошибках выше.
    echo.
    exit /b 1
)

echo.
echo =======================================================
echo  УСПЕХ! Приложение '%OUTPUT_NAME%' успешно скомпилировано.
echo =======================================================
echo.

endlocal