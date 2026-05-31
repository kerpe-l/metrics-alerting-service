// Кейс C: пакет НЕ main. Анализатор должен пропустить весь пакет,
// даже если в нём есть функция с именем main и os.Exit.
package c

import "os"

func main() {
	os.Exit(1)
}
