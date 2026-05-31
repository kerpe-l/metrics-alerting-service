// Кейс A: прямой вызов os.Exit в функции main пакета main — должен ловиться.
package main

import "os"

func main() {
	os.Exit(1) // want "прямой вызов os.Exit в функции main запрещён"
}
