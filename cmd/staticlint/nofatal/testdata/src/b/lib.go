// Кейс B: log.Fatal/Fatalf/Panicln в библиотечной функции — должны ловиться.
// Детект идёт по type info пакета log, не по строке.
package b

import "log"

// Boom демонстрирует недопустимые аварийные вызовы пакета log.
func Boom(cond bool) {
	if cond {
		log.Fatal("dead") // want "log.Fatal вне main/init запрещён"
	}
	if cond {
		log.Fatalf("dead %d", 1) // want "log.Fatalf вне main/init запрещён"
	}
	log.Panicln("dead") // want "log.Panicln вне main/init запрещён"
}
