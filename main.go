// Файл: main.go (Версия с зональным OCR, конфигом и локальной сборкой подписи)
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// --- Константы ---
const ocrWordLimit = 35

// --- Структуры для config.json ---
type Config struct {
	BotToken         string     `json:"bot_token"`
	ScreenshotTarget string     `json:"screenshot_target"`
	LogTarget        string     `json:"log_target"`
	OcrSpaceKeys     []string   `json:"ocr_space_keys"`
	AIProviders      []Provider `json:"ai_providers"`
}
type Provider struct {
	Name        string   `json:"name"`
	APIEndpoint string   `json:"api_endpoint"`
	ModelName   string   `json:"model_name"`
	Keys        []string `json:"keys"`
	Priority    int
}

// --- Структуры для AI ---
type ResponseFormat struct{ Type string `json:"type"` }
type APIRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	MaxTokens      int             `json:"max_tokens"`
	Temperature    float32         `json:"temperature"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}
type Message struct{ Role, Content string }
type APIResponse struct{ Choices []Choice }
type Choice struct{ Message Message }

type AI_Activity_Output struct {
	PrimaryProgram       string `json:"primary_program"`
	UserActivitySentence string `json:"user_activity_sentence"`
	ActivityCategory     string `json:"activity_category"`
}
type AI_Hydra_Output struct {
	TaskName   string `json:"task_name"`
	Percentage int    `json:"percentage"`
	Progress   string `json:"progress"`
}

// --- Структуры для OCR ---
type OcrResponse struct {
	ParsedResults         []ParsedResult `json:"ParsedResults"`
	OCRExitCode           int            `json:"OCRExitCode"`
	IsErroredOnProcessing bool           `json:"IsErroredOnProcessing"`
	ErrorMessage          string         `json:"ErrorMessage"`
}
type ParsedResult struct{ ParsedText string `json:"ParsedText"` }

// --- Функции-обертки для платформо-зависимого кода ---
func getScreenshot() (*image.RGBA, error) {
	return captureScreen()
}

func getHydraScreenshot() ([]byte, error) {
	img, err := getScreenshot()
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width < 600 || height < 200 {
		return nil, fmt.Errorf("разрешение экрана слишком мало для обрезки зоны 600x200")
	}

	cropRect := image.Rect(width-600, height-200, width, height)

	croppedImg, ok := img.SubImage(cropRect).(*image.RGBA)
	if !ok {
		return nil, fmt.Errorf("не удалось обрезать изображение")
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, croppedImg, nil); err != nil {
		return nil, fmt.Errorf("ошибка кодирования обрезанного JPEG: %v", err)
	}
	return buf.Bytes(), nil
}

func getWindowTitles() ([]string, error) {
	return getAllVisibleWindowTitles()
}

// --- Логика распознавания текста ---
func getTextFromImage(imageData []byte, ocrApiKeys []string, logFunc func(string)) (string, error) {
	if len(ocrApiKeys) == 0 || (len(ocrApiKeys) == 1 && ocrApiKeys[0] == "") {
		return "", fmt.Errorf("API ключ для OCR не предоставлен")
	}
	url := "https://api.ocr.space/parse/image"
	var lastError error
	for _, apiKey := range ocrApiKeys {
		var requestBody bytes.Buffer
		writer := multipart.NewWriter(&requestBody)
		part, err := writer.CreateFormFile("file", "screenshot.jpg")
		if err != nil {
			lastError = err
			continue
		}
		_, err = io.Copy(part, bytes.NewReader(imageData))
		if err != nil {
			lastError = err
			continue
		}
		writer.WriteField("apikey", apiKey)
		writer.WriteField("language", "rus")
		writer.WriteField("scale", "true")
		writer.Close()
		req, err := http.NewRequest("POST", url, &requestBody)
		if err != nil {
			lastError = err
			continue
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		client := &http.Client{Timeout: 45 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			lastError = err
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			lastError = fmt.Errorf("OCR.space API вернул ошибку: %s (тело: %s)", resp.Status, string(bodyBytes))
			continue
		}
		var ocrResponse OcrResponse
		if err := json.NewDecoder(resp.Body).Decode(&ocrResponse); err != nil {
			lastError = err
			continue
		}
		if ocrResponse.IsErroredOnProcessing {
			lastError = fmt.Errorf("ошибка обработки в OCR.space: %s", ocrResponse.ErrorMessage)
			continue
		}
		if len(ocrResponse.ParsedResults) > 0 && ocrResponse.ParsedResults[0].ParsedText != "" {
			return ocrResponse.ParsedResults[0].ParsedText, nil
		}
	}
	logFunc(fmt.Sprintf("⚠️ OCR: Не удалось получить текст с изображения: %v", lastError))
	return "", lastError
}

// --- Логика анализа AI ---
func analyzeHydraTask(ocrText string, providers []Provider, logFunc func(string)) (*AI_Hydra_Output, error) {
	prompt := fmt.Sprintf(`
ТЫ — АНАЛИТИК. Извлеки данные о прогрессе из текста.

ЗАДАНИЕ: Верни JSON с 3 ключами:
- "task_name": (string) Название задачи.
- "percentage": (int) Процент (только число).
- "progress": (string) Прогресс (формат "X of Y").

ТЕКСТ:
%s`, ocrText)

	logFunc(fmt.Sprintf("---\nДанные для AI (Гидра):\n%s\n---", ocrText))
	// ... (Далее логика запросов к API, аналогичная analyzeGeneralActivity, но для структуры AI_Hydra_Output)
}

func analyzeGeneralActivity(titles []string, truncatedOcrText string, providers []Provider, logFunc func(string)) (*AI_Activity_Output, error) {
	prompt := fmt.Sprintf(`
ТЫ — АНАЛИТИК. Определи задачу пользователя по данным.

ПРИОРИТЕТЫ:
1. Заголовки окон (главное).
2. OCR-текст (контекст).

ЗАДАНИЕ: Верни JSON с 3 ключами:
- "primary_program": (string) Главная программа из заголовков.
- "user_activity_sentence": (string) Детальное описание задачи (10-15 слов на русском).
- "activity_category": (string) Категория (Разработка, Коммуникация, Дизайн, Веб-серфинг, Гейминг, Офисная работа, Мультимедиа, Системные задачи).

ДАННЫЕ:
1. Заголовки окон:
%s
2. OCR-текст (контекст, до %d слов):
%s`, strings.Join(titles, "\n"), ocrWordLimit, truncatedOcrText)

	logFunc(fmt.Sprintf("---\nДанные для AI (Общий анализ):\nЗаголовки: %s\nOCR: %s\n---", strings.Join(titles, "\n"), truncatedOcrText))
	// ... (Далее логика запросов к API, аналогичная старому analyzeContent, но для структуры AI_Activity_Output)
}

// --- Вспомогательные функции ---
func loadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть файл конфигурации: %w", err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err = decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("не удалось декодировать JSON: %w", err)
	}

	for i := range config.AIProviders {
		config.AIProviders[i].Priority = i
	}

	return &config, nil
}

func truncateTextByWords(text string, limit int) string {
	words := strings.Fields(text)
	if len(words) <= limit {
		return text
	}
	return strings.Join(words[:limit], " ")
}

// --- Логика отправки в Telegram ---
func sendLog(bot *tgbotapi.BotAPI, logTargetInfo, errorMessage string) {
	if bot == nil || logTargetInfo == "" {
		return
	}
	parts := strings.Split(logTargetInfo, ":")
	if len(parts) != 2 {
		return
	}
	groupID, errGroup := strconv.ParseInt(parts[0], 10, 64)
	topicID, errTopic := strconv.Atoi(parts[1])
	if errGroup != nil || errTopic != nil {
		return
	}
	fullMessage := fmt.Sprintf("📝 Лог ScreenSender:\n\n%s", errorMessage)
	msg := tgbotapi.NewMessage(groupID, fullMessage)
	msg.ReplyToMessageID = topicID
	bot.Send(msg)
}

func processAndSend(bot *tgbotapi.BotAPI, screenshotTargetInfo string, photoData []byte, caption string, logFunc func(string)) {
	if bot == nil || screenshotTargetInfo == "" {
		logFunc("ID для скриншотов не указан.")
		return
	}
	parts := strings.Split(screenshotTargetInfo, ":")
	if len(parts) != 2 {
		logFunc(fmt.Sprintf("Неверный формат ID для скриншотов: %s", screenshotTargetInfo))
		return
	}
	groupID, errGroup := strconv.ParseInt(parts[0], 10, 64)
	topicID, errTopic := strconv.Atoi(parts[1])
	if errGroup != nil || errTopic != nil {
		logFunc("Не удалось распарсить ID для скриншотов.")
		return
	}

	file := tgbotapi.FileBytes{Name: "screenshot.jpg", Bytes: photoData}
	msg := tgbotapi.NewPhoto(groupID, file)
	msg.Caption = caption
	msg.ReplyToMessageID = topicID
	if _, err := bot.Send(msg); err != nil {
		logFunc(fmt.Sprintf("Ошибка отправки скриншота: %v", err))
	}
}

// --- Главная функция ---
func main() {
	log.Println("🚀 Запуск ScreenSender...")

	config, err := loadConfig("config.json")
	if err != nil {
		log.Fatalf("КРИТИЧЕСКАЯ ОШИБКА: %v", err)
	}

	if config.BotToken == "" || config.ScreenshotTarget == "" {
		log.Fatal("КРИТИЧЕСКАЯ ОШИБКА: 'bot_token' и 'screenshot_target' не могут быть пустыми в config.json.")
	}

	bot, err := tgbotapi.NewBotAPI(config.BotToken)
	if err != nil {
		log.Fatalf("Ошибка инициализации бота: %v", err)
	}

	logAndSend := func(errMsg string) {
		log.Println(errMsg)
		sendLog(bot, config.LogTarget, errMsg)
	}

	// --- ШАГ 1: Зональный OCR для "Гидры" ---
	logAndSend("Шаг 1: Проверка зоны 'Гидра' (600x200, правый нижний угол).")
	hydraImageData, err := getHydraScreenshot()
	if err != nil {
		logAndSend(fmt.Sprintf("⚠️ Не удалось получить скриншот для зоны 'Гидра': %v", err))
	} else {
		hydraOcrText, err := getTextFromImage(hydraImageData, config.OcrSpaceKeys, logAndSend)
		if err != nil {
			logAndSend(fmt.Sprintf("⚠️ OCR для зоны 'Гидра' не удался: %v", err))
		} else if strings.Contains(hydraOcrText, "%") || len(strings.Fields(hydraOcrText)) > 2 { // Более надежная проверка
			logAndSend("✅ Возможная задача 'Гидра' найдена! Анализируем прогресс...")
			hydraResult, err := analyzeHydraTask(hydraOcrText, config.AIProviders, logAndSend)
			if err != nil || hydraResult == nil {
				logAndSend(fmt.Sprintf("❌ Не удалось проанализировать прогресс 'Гидры': %v. Переход к общему анализу.", err))
			} else {
				fullScreenshotImg, _ := getScreenshot()
				var buf bytes.Buffer
				jpeg.Encode(&buf, fullScreenshotImg, nil)

				caption := fmt.Sprintf("Задача: %s\nПрогресс: %d%% (%s)",
					hydraResult.TaskName, hydraResult.Percentage, hydraResult.Progress)

				processAndSend(bot, config.ScreenshotTarget, buf.Bytes(), caption, logAndSend)
				logAndSend("✅ Задача 'Гидра' выполнена. Программа завершает работу.")
				return
			}
		} else {
			logAndSend("ℹ️ 'Гидра' не найдена в указанной зоне. Переход к общему анализу.")
		}
	}

	// --- ШАГ 2: Общий анализ (если "Гидра" не найдена) ---
	logAndSend("Шаг 2: Общий анализ активности.")
	fullScreenshotImg, err := getScreenshot()
	if err != nil {
		logAndSend(fmt.Sprintf("❌ Не удалось сделать скриншот: %v", err))
		return
	}
	titles, err := getWindowTitles()
	if err != nil {
		logAndSend(fmt.Sprintf("⚠️ Не удалось получить заголовки окон: %v", err))
		titles = []string{}
	}

	var buf bytes.Buffer
	jpeg.Encode(&buf, fullScreenshotImg, nil)
	fullScreenshotBytes := buf.Bytes()

	fullOcrText, _ := getTextFromImage(fullScreenshotBytes, config.OcrSpaceKeys, logAndSend)
	truncatedOcrText := truncateTextByWords(fullOcrText, ocrWordLimit)

	analysisResult, err := analyzeGeneralActivity(titles, truncatedOcrText, config.AIProviders, logAndSend)
	var caption string
	if err != nil || analysisResult == nil {
		caption = "Активность не определена"
	} else {
		caption = analysisResult.UserActivitySentence
	}

	processAndSend(bot, config.ScreenshotTarget, fullScreenshotBytes, caption, logAndSend)
	logAndSend("✅ Общая задача выполнена. Программа завершает работу.")
}