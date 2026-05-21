# go-musthave-metrics-tpl

Шаблон репозитория для трека «Сервер сбора метрик и алертинга».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-metrics-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**

## Профилирование и оптимизация

### Как воспроизвести

1. Запустить сервер с pprof-эндпоинтом:
   ```
   ./server -a 127.0.0.1:18080 -i 0 -f "" -pprof 127.0.0.1:16060
   ```
2. Подать нагрузку (`payload.json` — батч из ~50 метрик):
   ```
   ab -n 20000 -c 50 -p payload.json -T application/json \
      -H "Accept-Encoding: gzip" http://127.0.0.1:18080/updates/
   ```
3. Снять профиль аллокаций:
   ```
   curl -o profiles/base.pprof http://127.0.0.1:16060/debug/pprof/allocs
   ```

Профили: [`profiles/base.pprof`](profiles/base.pprof), [`profiles/result.pprof`](profiles/result.pprof).

### Что сделано

- `internal/gzip`: `gzip.Writer` и `gzip.Reader` теперь берутся из `sync.Pool` и переиспользуются между запросами. Это убрало основной источник аллокаций — `flate.NewWriter` на каждый сжатый ответ.
- `internal/handler.RootHandler`: вместо квадратичного `html += fmt.Sprintf(...)` используется `strings.Builder` с предварительной оценкой ёмкости.
- `internal/hash`: `hmac.Hash` кэшируется в `sync.Pool` (по ключу), `bytes.Buffer` для перехвата тела ответа также пулится; в `Compute` сумма кладётся в стек-массив, чтобы избежать allocation в `hash.Sum`.
- `internal/agent.Sender`: `gzip.Writer` переиспользуется через `sync.Pool`.

### Diff профилей

```
$ go tool pprof -top -diff_base=profiles/base.pprof profiles/result.pprof
File: metrics-server
Type: alloc_space
Showing nodes accounting for -4473.46MB, 87.58% of 5107.62MB total
Dropped 138 nodes (cum <= 25.54MB)
      flat  flat%   sum%        cum   cum%
-3119.39MB 61.07% 61.07% -3793.88MB 74.28%  compress/flate.NewWriter (inline)
 -657.98MB 12.88% 73.96%  -657.98MB 12.88%  compress/flate.(*compressor).initDeflate (inline)
 -264.33MB  5.18% 79.13% -4078.73MB 79.86%  internal/handler.(*MetricsHandler).RootHandler
 -176.59MB  3.46% 82.59%  -176.59MB  3.46%  reflect.growslice
 -120.37MB  2.36% 84.94%  -119.87MB  2.35%  encoding/json.(*Decoder).refill
  -32.07MB  0.63% 85.57%   -32.07MB  0.63%  compress/flate.(*huffmanEncoder).generate
  -30.10MB  0.59% 86.16%   -30.10MB  0.59%  bufio.NewWriterSize (inline)
  -29.11MB  0.57% 86.73%   -29.11MB  0.57%  bufio.NewReaderSize (inline)
  -20.02MB  0.39% 87.12%  -338.98MB  6.64%  internal/handler.(*MetricsHandler).UpdateBatchHandler
     -10MB   0.2% 87.32% -4467.79MB 87.47%  internal/logger.RequestLogger.func1
   -8.50MB  0.17% 87.49%   -38.03MB  0.74%  net/http.(*conn).readRequest
   -2.50MB 0.049% 87.54% -4584.53MB 89.76%  net/http.(*conn).serve
   -2.50MB 0.049% 87.58% -4452.28MB 87.17%  internal/gzip.Middleware.func1
```

Отрицательные значения подтверждают снижение аллокаций. Суммарно — **−4473 MB (≈ 87% от исходных)**; основная экономия пришлась на `compress/flate.NewWriter` и `RootHandler`.

### Бенчмарки

```
$ go test -run=^$ -bench=. -benchmem -benchtime=200ms \
    ./internal/handler ./internal/repository ./internal/agent ./internal/hash ./internal/gzip
```

| Бенчмарк | до | после |
|---|---|---|
| `BenchmarkRootHandler` | 254737 ns · 2326082 B · 1437 allocs | 18730 ns · 69082 B · 130 allocs |
| `BenchmarkMiddleware_CompressJSON` (gzip) | 46145 ns · 821139 B · 40 allocs | 17079 ns · 7546 B · 23 allocs |
| `BenchmarkCompute` (hmac-sha256) | 578 ns · 656 B · 9 allocs | 443 ns · 160 B · 3 allocs |
| `BenchmarkMiddleware` (hash) | 2600 ns · 10976 B · 52 allocs | 2304 ns · 9904 B · 39 allocs |
