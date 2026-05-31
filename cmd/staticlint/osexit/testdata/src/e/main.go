// Кейс E: импорт os под алиасом. Проверяем, что детект идёт через type info,
// а не строковое сравнение «os.Exit».
package main

import xos "os"

func main() {
	xos.Exit(1) // want "прямой вызов os.Exit в функции main запрещён"
}
