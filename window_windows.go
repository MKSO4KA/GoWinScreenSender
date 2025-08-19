// Файл: window_windows.go (ВЕРСИЯ С УЛУЧШЕННЫМ ПОЛУЧЕНИЕМ ВСЕХ ОКОН)
package main

import (
	"strings"
	"syscall"
	"unsafe"
)

// --- Windows API функции ---
var (
	procEnumWindows          = user32.NewProc("EnumWindows")
	procIsWindowVisible      = user32.NewProc("IsWindowVisible")
	procGetWindowTextW       = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	procGetWindow            = user32.NewProc("GetWindow")
	procGetParent            = user32.NewProc("GetParent")
)

// Константа для проверки
const (
	GW_OWNER = 4
)

var enumWindowsCallback = syscall.NewCallback(func(hwnd syscall.Handle, lParam uintptr) uintptr {
	// 1. Проверяем, видимо ли окно. Это основное условие.
	isVisible, _, _ := procIsWindowVisible.Call(uintptr(hwnd))
	if isVisible == 0 {
		return 1 // Продолжить перечисление
	}

	// 2. Проверяем, есть ли у окна заголовок. Окна без заголовка нам не нужны.
	textLen, _, _ := procGetWindowTextLengthW.Call(uintptr(hwnd))
	if textLen == 0 {
		return 1 // Продолжить
	}

	// 3. УМНЫЙ ФИЛЬТР: отсеиваем дочерние окна и окна, у которых есть "владелец".
	// Это позволяет нам получить только окна верхнего уровня (главные окна приложений).
	parent, _, _ := procGetParent.Call(uintptr(hwnd))
	owner, _, _ := procGetWindow.Call(uintptr(hwnd), GW_OWNER)
	if parent != 0 || owner != 0 {
		return 1 // Продолжить
	}

	// 4. Получаем текст заголовка окна
	buffer := make([]uint16, textLen+1)
	procGetWindowTextW.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(textLen+1),
	)
	title := syscall.UTF16ToString(buffer)

	// <<< --- ВАШИ СПЕЦИАЛЬНЫЕ ФИЛЬТРЫ --- >>>
	// Исключаем системное окно Program Manager и окна отладчика AnyDesk
	if title == "Program Manager" || strings.Contains(title, "AnyDesk") {
		return 1 // Продолжить, пропуская это окно
	}
	// <<< --- КОНЕЦ ФИЛЬТРОВ --- >>>

	// 5. Если все проверки пройдены, добавляем заголовок в наш срез
	titles := (*[]string)(unsafe.Pointer(lParam))
	*titles = append(*titles, title)

	return 1 // Обязательно возвращаем 1, чтобы продолжить перечисление
})

func getAllVisibleWindowTitles() ([]string, error) {
	var titles []string
	// Запускаем перечисление всех окон верхнего уровня на рабочем столе
	procEnumWindows.Call(enumWindowsCallback, uintptr(unsafe.Pointer(&titles)))
	return titles, nil
}
