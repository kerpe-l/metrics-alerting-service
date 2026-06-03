// Кейс A: panic в обычной библиотечной функции — должен ловиться.
package a

// Risky демонстрирует недопустимый panic в библиотечном коде.
func Risky() {
	panic("boom") // want "panic вне main/init запрещён"
}
