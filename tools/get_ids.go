// Файл: get_ids.go (ВЕРСИЯ ДЛЯ СТАРЫХ БИБЛИОТЕК)
package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Вспомогательная функция, чтобы не повторять код (версия для старых библиотек)
func getTopicInfo(bot *tgbotapi.BotAPI, updates tgbotapi.UpdatesChannel, topicName string) (string, error) {
	log.Printf("--- ШАГ: Получение ID для темы '%s' ---", topicName)
	log.Printf("Напишите ЛЮБОЕ сообщение в тему (топик) под названием '%s'.", topicName)
	log.Println("Ожидаю сообщение (есть 90 секунд)...")

	timeout := time.After(90 * time.Second)

	for {
		select {
		case update := <-updates:
			// В старых версиях библиотеки нет поля MessageThreadID.
			// Проверяем, что сообщение является ответом на другое сообщение.
			if update.Message != nil && update.Message.ReplyToMessage != nil {
				
				// ID темы (топика) в данном случае - это ID исходного сообщения, на которое отвечают.
				// Когда вы пишете в тему, ваше сообщение технически является ответом на 
				// невидимое "сервисное" сообщение о создании темы.
				topicID := update.Message.ReplyToMessage.MessageID
				groupID := update.Message.Chat.ID
				
				log.Printf("Получено сообщение в группе '%s'", update.Message.Chat.Title)
				
				fmt.Printf("\n✅ УСПЕХ! Данные для темы '%s' получены.\n", topicName)
				fmt.Printf("   ID Группы: %d\n", groupID)
				fmt.Printf("   ID Темы:   %d\n\n", topicID)

				return strconv.FormatInt(groupID, 10) + ":" + strconv.Itoa(topicID), nil
			}
		case <-timeout:
			return "", fmt.Errorf("за 90 секунд не было получено сообщения в теме")
		}
	}
}

func main() {
	fmt.Println("Введите токен вашего Telegram-бота и нажмите Enter:")
	var botToken string
	fmt.Scanln(&botToken)
	if botToken == "" {
		log.Fatal("Токен не может быть пустым.")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Ошибка инициализации бота: %v", err)
	}

	log.Printf("Авторизован как @%s", bot.Self.UserName)
	log.Println("---")
	log.Println("Инструкция:")
	log.Println("1. Добавьте вашего бота (@%s) в нужную группу.", bot.Self.UserName)
	log.Println("2. Убедитесь, что в группе есть две темы (например, 'Скриншоты' и 'Логи').")
	log.Println("3. Следуйте инструкциям для каждой темы.")
	
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	// Очищаем старые "зависшие" апдейты
	time.Sleep(time.Millisecond * 500)
	for len(updates) > 0 {
		<-updates
	}
	
	screenshotTarget, err := getTopicInfo(bot, updates, "Скриншоты")
	if err != nil {
		log.Fatalf("Не удалось получить ID для темы 'Скриншоты': %v", err)
	}

	logTarget, err := getTopicInfo(bot, updates, "Логи")
	if err != nil {
		log.Fatalf("Не удалось получить ID для темы 'Логи': %v", err)
	}

	fmt.Println("==============================================")
	fmt.Println("✅ ВСЕ ДАННЫЕ ПОЛУЧЕНЫ!")
	fmt.Println("==============================================")
	fmt.Println("Готовые строки для ввода в build.sh (СКОПИРУЙТЕ ИХ):")
	fmt.Println("\n--- Для СКРИНШОТОВ ---")
	fmt.Println(screenshotTarget)
	fmt.Println("\n--- Для ЛОГОВ ---")
	fmt.Println(logTarget)
	fmt.Println("\nСкрипт завершает работу.")
}
