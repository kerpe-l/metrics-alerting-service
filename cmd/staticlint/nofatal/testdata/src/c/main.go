// Кейс C: func main пакета main — легальный сток, panic и log.Fatal разрешены.
package main

import "log"

func main() {
	if false {
		panic("ok in main")
	}
	log.Fatal("ok in main")
}
