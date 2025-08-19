// Файл: test_script.go (ВЕРСЯ ДЛЯ ТЕСТА "УМНОГО" ПРОМПТА С АКЦЕНТОМ НА OCR)
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// --- Структуры для конфига ---
type Config struct {
	BotToken  string  `json:"bot_token"`
	LogTarget string  `json:"log_target"`
	APIKeys   APIKeys `json:"api_keys"`
}
type APIKeys struct {
	NavyAI      []string `json:"navy_ai"`
	ElectronHub []string `json:"electron_hub"`
	VoidAI      []string `json:"void_ai"`
}

type Provider struct{ Name, APIEndpoint, ModelName string; Keys []string }

// --- Структуры для запроса и ответа AI ---
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

func loadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil { return nil, err }
	defer file.Close()
	var config Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	return &config, err
}

func sendAIRequest(provider Provider, key, prompt string) (string, error) {
	payload := APIRequest{
		Model:          provider.ModelName,
		Messages:       []Message{{Role: "user", Content: prompt}},
		MaxTokens:      500,
		Temperature:    0.3,
		ResponseFormat: &ResponseFormat{Type: "json_object"},
	}
	reqBody, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", provider.APIEndpoint, bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("статус: %s, тело: %s", resp.Status, string(body))
	}

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil { return "", err }
	if len(apiResp.Choices) > 0 { return apiResp.Choices[0].Message.Content, nil }
	return "", fmt.Errorf("пустой ответ от API")
}

func main() {
	log.Println("--- [ Запуск тестового скрипта ] ---")
	config, err := loadConfig("config.json")
	if err != nil {
		log.Fatalf("Критическая ошибка: не удалось прочитать config.json: %v", err)
	}

	log.Println("\n--- [ Тестирование AI API с 'умным' промптом и акцентом на OCR в JSON Mode ] ---")

	// 1. Данные из вашего лога
	titles := []string{
		"Steam",
	}
	ocrText := "5 meet.goog]e.com Бармен Боб хуЈКаРиК 19:04 Азре1Г- «о • 000 Илья 40 40 сек. Бодрая гармония Пассивный эффект: в конце каждого 3-го хода вы получаете"
	ocrWordLimit := 25

	// 2. Урезанный текст, как в main.go
	words := strings.Fields(ocrText)
	if len(words) > ocrWordLimit {
		ocrText = strings.Join(words[:ocrWordLimit], " ")
	}
	
	// 3. Промпт, в точности как в main.go
	realisticPrompt := fmt.Sprintf(`
ТЫ — ЭКСПЕРТ-АНАЛИТИК. Твоя цель — определить задачу пользователя.

АНАЛИТИЧЕСКИЕ ПРИОРИТЕТЫ:
1.  **Текст с экрана (OCR Text) является самым важным доказательством.** Он показывает, на что пользователь смотрит или что пишет в данный момент.
2.  **Ищи признаки многозадачности.** Если OCR-текст упоминает инструменты для общения (Google Meet, Discord, Slack, Telegram), а заголовок окна — это игра или редактор кода, пользователь, скорее всего, совмещает дела. Отрази это в своем выводе.

ЗАДАНИЕ:
Проанализируй данные и верни JSON-объект со следующими 4 ключами:
- "primary_program": (string) Название наиболее активной программы.
- "user_activity_sentence": (string) Подробное описание задачи пользователя (5-10 слов на русском).
- "activity_category": (string) Одна из категорий: Разработка, Коммуникация, Дизайн, Веб-серфинг, Гейминг, Офисная работа, Мультимедиа, Системные задачи.
- "reasoning": (string) Краткое (1 предложение) объяснение твоего вывода.

ВХОДНЫЕ ДАННЫЕ:
1. Заголовки окон:
%s

2. Текст с экрана (до %d слов):
%s`, strings.Join(titles, "\n"), ocrWordLimit, ocrText)

	log.Println("--- Сформирован промпт с акцентом на многозадачность. Тестируем... ---")

	providers := []Provider{
		{"NavyAI", "https://api.navy/v1/chat/completions", "gpt-4o", config.APIKeys.NavyAI},
		{"ElectronHub", "https://api.electronhub.ai/v1/chat/completions", "gemini-1.5-flash-latest", config.APIKeys.ElectronHub},
		{"VoidAI", "https://api.voidai.app/v1/chat/completions", "gpt-3.5-turbo", config.APIKeys.VoidAI},
	}

	for _, p := range providers {
		log.Printf("\n-> Проверка провайдера: %s", p.Name)
		if len(p.Keys) == 0 {
			log.Println("   Пропущено: ключи не предоставлены.")
			continue
		}
		success := false
		for _, key := range p.Keys {
			shortKey := key
			if len(key) > 8 {
				shortKey = key[:4] + "..." + key[len(key)-4:]
			}
			log.Printf("   - Пытаюсь использовать ключ: %s", shortKey)
			response, err := sendAIRequest(p, key, realisticPrompt)
			if err != nil {
				log.Printf("     ❌ ОШИБКА: %v", err)
			} else {
				log.Printf("     ✅ УСПЕХ! Ответ API:\n%s", strings.TrimSpace(response))
				success = true
				break
			}
		}
		if !success {
			log.Printf("   ❌ ИТОГ: Ни один из ключей для %s не сработал.", p.Name)
		}
	}
	
	log.Println("\n--- [ Тестирование завершено ] ---")
}
