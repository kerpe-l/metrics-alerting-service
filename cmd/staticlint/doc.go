// Команда staticlint — multichecker для статического анализа проекта.
//
// # Назначение
//
// Бинарник собирает в единый запуск стандартные анализаторы Go, полный набор
// SA-проверок staticcheck.io, проверки classes simple/stylecheck/quickfix,
// два публичных сторонних анализатора и собственные анализаторы osexit и
// nofatal. Используется как локально, так и в CI поверх всего модуля.
//
// # Запуск
//
// Сборка и запуск:
//
//	go build -o bin/staticlint ./cmd/staticlint
//	./bin/staticlint ./...
//
// Либо без сборки:
//
//	go run ./cmd/staticlint ./...
//
// Multichecker поддерживает стандартные флаги фреймворка
// golang.org/x/tools/go/analysis:
//
//	./bin/staticlint help                 # справка по всем анализаторам и флагам
//	./bin/staticlint -V=full              # информация о версии
//	./bin/staticlint -<name> ./...        # запустить только указанный анализатор
//	./bin/staticlint -<name>=false ./...  # отключить указанный анализатор
//
// Например, чтобы запустить только собственный анализатор:
//
//	./bin/staticlint -osexit ./...
//
// # Состав
//
// 1. Стандартные анализаторы из golang.org/x/tools/go/analysis/passes:
//   - assign        — выявляет бесполезные присваивания (a = a).
//   - atomic        — некорректное использование sync/atomic.
//   - bools         — подозрительные операции с булевыми выражениями.
//   - buildtag      — корректность build-тегов.
//   - composite     — литералы составных типов без имён полей.
//   - copylock      — копирование структур с sync.Mutex и т.п.
//   - errorsas      — второй аргумент errors.As не должен быть *error.
//   - httpresponse  — использование *http.Response до проверки ошибки.
//   - loopclosure   — захват переменной цикла в горутине/замыкании.
//   - lostcancel    — потерянный context.CancelFunc.
//   - nilfunc       — сравнение функции с nil без вызова.
//   - printf        — корректность форматных строк fmt.*.
//   - shadow        — затенение переменных во вложенных областях.
//   - shift         — сдвиги, превышающие ширину типа.
//   - stdmethods    — сигнатуры методов стандартных интерфейсов (Error и т.п.).
//   - structtag     — корректность struct-тегов.
//   - tests         — типичные ошибки в файлах _test.go.
//   - unmarshal     — передача не-указателя в Unmarshal.
//   - unreachable   — недостижимый код.
//   - unsafeptr     — неверные преобразования через unsafe.Pointer.
//   - unusedresult  — игнорируемый результат «чистых» функций (errors.New и т.п.).
//
// 2. Все анализаторы класса SA из honnef.co/go/tools/staticcheck. Это набор
// проверок корректности: нерабочие конструкции, мёртвые ветки, ошибки API
// стандартной библиотеки и т.п. Подключаются итерацией по
// staticcheck.Analyzers с отбором по префиксу имени SA.
//
// 3. Анализаторы остальных классов staticcheck.io:
//   - simple     (S1xxx)  — упрощения кода без изменения поведения.
//   - stylecheck (ST1xxx) — стилистические рекомендации (например, ST1000:
//     каждый пакет должен иметь package-level doc-комментарий).
//   - quickfix   (QF1xxx) — предложения автозамен.
//
// 4. Публичные сторонние анализаторы:
//   - bodyclose (github.com/timakin/bodyclose/passes/bodyclose) — следит за
//     закрытием тела HTTP-ответа (*http.Response.Body). Актуально для HTTP-агента
//     и приёмников аудита.
//   - nilerr (github.com/gostaticanalysis/nilerr) — выявляет паттерн
//     «получили ошибку — вернули nil» и обратный («ошибки нет — вернули err»).
//
// 5. Собственные анализаторы:
//   - osexit (см. подпакет ./osexit) — запрещает прямой вызов os.Exit в функции
//     main пакета main. Импорт os под алиасом учитывается: детект идёт через
//     type info, а не по тексту.
//   - nofatal (см. подпакет ./nofatal) — запрещает panic и
//     log.Fatal/log.Panic вне функций main и init. Вместе с osexit даёт цельное
//     правило: os.Exit — нельзя нигде, panic/log.Fatal — только в точках входа,
//     библиотечный код возвращает ошибки. Детект — через type info.
package main
