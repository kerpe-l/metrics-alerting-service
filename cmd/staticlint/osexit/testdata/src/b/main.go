// Кейс B: os.Exit спрятан в другой функции, main её только зовёт.
// Анализатор должен молчать — область только тело main.
package main

import "os"

func quit() {
	os.Exit(1)
}

func main() {
	quit()
}
