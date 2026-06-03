// Кейс G: panic во вложенном литерале (в т.ч. горутине) внутри main —
// тишина, литерал считается частью main (консистентно с osexit).
package main

func main() {
	f := func() {
		panic("nested in main")
	}
	_ = f
}
