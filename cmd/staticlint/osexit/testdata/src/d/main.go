// Кейс D: чистый main без os.Exit — ничего не репортим.
package main

import "fmt"

func main() {
	fmt.Println("ok")
}
