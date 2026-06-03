// Кейс F: метод с receiver — не main и не init, panic должен ловиться.
package f

// T — тип с методом, нарушающим правило.
type T struct{}

// Do демонстрирует недопустимый panic в методе.
func (T) Do() {
	panic("no") // want "panic вне main/init запрещён"
}
