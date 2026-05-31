# cmd/staticlint

Multichecker для статического анализа проекта. Собирает в один бинарник:

- стандартные анализаторы `golang.org/x/tools/go/analysis/passes/*`;
- все анализаторы класса `SA` из `honnef.co/go/tools/staticcheck`;
- анализаторы остальных классов staticcheck: `simple`, `stylecheck`, `quickfix`;
- публичные сторонние: `bodyclose`, `nilerr`;
- собственный [`osexit`](./osexit) — запрет прямого вызова `os.Exit` в `func main` пакета `main`.

## Запуск

```sh
go run ./cmd/staticlint ./...
# или
go build -o bin/staticlint ./cmd/staticlint && ./bin/staticlint ./...
```

Справка по анализаторам и флагам:

```sh
./bin/staticlint help
```

Подробное описание состава и каждого анализатора — в [godoc пакета](./doc.go).
