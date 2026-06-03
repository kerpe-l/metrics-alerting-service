// Package buildinfo печатает сведения о сборке (версия, дата, коммит).
//
// Значения подставляются линкером через -ldflags "-X main.buildVersion=...".
// Пустые значения заменяются на "N/A".
package buildinfo

import (
	"fmt"
	"io"
)

// naString — заглушка для незаданного значения.
const naString = "N/A"

// Fprint пишет в w три строки со сведениями о сборке. Пустые значения
// заменяются на "N/A".
func Fprint(w io.Writer, version, date, commit string) error {
	_, err := fmt.Fprintf(w,
		"Build version: %s\nBuild date: %s\nBuild commit: %s\n",
		orNA(version), orNA(date), orNA(commit),
	)
	return err
}

// orNA возвращает s либо "N/A", если строка пустая.
func orNA(s string) string {
	if s == "" {
		return naString
	}
	return s
}
