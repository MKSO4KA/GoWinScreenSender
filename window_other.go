//go:build !windows

// Файл: window_other.go
package main

import "fmt"

// Функция-заглушка, чтобы сборка проходила на других системах
func getAllVisibleWindowTitles() ([]string, error) {
	return nil, fmt.Errorf("перечисление окон доступно только под Windows")
}
