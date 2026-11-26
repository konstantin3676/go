# Gateway

Gateway предоставляет HTTP API для бюджетов, расходов и сводного отчёта.
Данные сохраняются Ledger в PostgreSQL, а отчёты кешируются Ledger в Redis.

## Инициализация и зависимости

Gateway получает готовый `ledger.Service` через фабрику `ledger.New(ctx)`:

```go
svc, closeFn, err := ledger.New(startupCtx)
if err != nil {
    return fmt.Errorf("initialize Ledger: %w", err)
}
defer closeFn()

handler := api.LoggingMiddleware(api.TimeoutMiddleware(timeout, api.NewHandler(svc).Routes()))
```

Этот код находится в функции `run`; ошибка возвращается в `main`, логируется,
и процесс завершается. На инициализацию выделено 10 секунд.
Конфигурация, `sql.Open`, PostgreSQL, Redis и сборка репозиториев скрыты в Ledger.
Gateway не импортирует драйверы БД и пакеты репозиториев и не запускает миграции.
Отдельный процесс Ledger для работы Gateway не требуется.

Обработчики вызывают методы сервиса с `r.Context()`.
Валидацию сущностей и бюджетное правило выполняет сервис, HTTP-слой отвечает
за JSON, параметры запроса и отображение ошибок: `400` — валидация,
`409` — превышение бюджета, `500` — внутренняя ошибка,
`504` — таймаут или отмена операции.
Формат ошибки: `{"error":"описание ошибки"}`.

## Запуск

Основной способ запуска описан в [корневом README](../README.md#запуск-через-docker-compose):
запустите PostgreSQL и Redis, примените миграции Goose, затем запустите Gateway.

Для локального запуска через Go подготовьте [PostgreSQL и Redis](../README.md#postgresql-и-redis-для-локального-запуска-без-compose),
затем выполните из корня репозитория:

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/cashapp?sslmode=disable'
export REDIS_ADDR=localhost:6379
export REDIS_DB=0
export REDIS_PASSWORD=''

goose -dir ./ledger/migrations postgres "$DATABASE_URL" up

cd gateway
go run .
```

Gateway доступен на `http://localhost:8080`. Команда
`curl -fsS http://localhost:8080/ping` должна вывести `pong`.

PostgreSQL и Redis должны быть доступны до запуска. При ошибке подключения
сервер не стартует. SIGINT/SIGTERM запускает остановку HTTP-сервера с ожиданием
текущих запросов до 5 секунд, после чего освобождаются ресурсы Ledger.

## Проверка API

Пример рассчитан на категорию без прежних расходов. В корневом README есть
[быстрая проверка API с пакетным импортом](../README.md#быстрая-проверка-api)
и [сценарий с проверкой перезапуска](../README.md#проверка-сценария-задания).

```bash
curl -X POST http://localhost:8080/api/budgets \
  -H 'Content-Type: application/json' \
  -d '{"category":"еда","limit":1000}'

curl -X POST http://localhost:8080/api/transactions \
  -H 'Content-Type: application/json' \
  -d '{"amount":450,"category":"еда","description":"ланч","date":"2025-09-10"}'

curl -i -X POST http://localhost:8080/api/transactions \
  -H 'Content-Type: application/json' \
  -d '{"amount":600,"category":"еда","description":"ужин","date":"2025-09-11"}'

curl http://localhost:8080/api/transactions
curl http://localhost:8080/api/budgets

curl 'http://localhost:8080/api/reports/summary?from=2025-09-01&to=2025-09-30'
curl 'http://localhost:8080/api/reports/summary?from=2025-09-01&to=2025-09-30'
```

Превышающая лимит транзакция возвращает `409 Conflict`. Списки бюджетов
кешируются в Redis на 20 секунд и кеш сбрасывается при изменении бюджета.
Повторный запрос отчёта в течение 30 секунд получает результат из Redis.

### Сводный отчёт

`GET /api/reports/summary?from=2025-09-01&to=2025-09-30`
возвращает объект сумм по категориям, например:

```json
{"еда":1234.56,"транспорт":789}
```

Без расходов ответ — `{}`. Обе даты обязательны, формат — `YYYY-MM-DD`,
`from <= to`, границы включены. Ошибки параметров возвращают `400` и JSON-ошибку.

## Таймауты запросов

Middleware применяется ко всем API-ручкам, создаёт `context.WithTimeout` и
передаёт его через `r.WithContext(ctx)`. По умолчанию таймаут — **2 секунды**.
Изменение требует перезапуска Gateway:

```bash
REQUEST_TIMEOUT=50ms go run .
# Вернуть обычное значение после проверки:
REQUEST_TIMEOUT=2s go run .
```

Пустая или отсутствующая переменная означает `2s`; неверное, нулевое или
отрицательное значение — ошибка запуска. При отмене или истечении времени
возвращается `504 Gateway Timeout` с JSON:

```json
{"error":"request timed out or canceled"}
```

Отмена кооперативная: SQL и вычисления используют контекст, перед ответом
завершаются работники и heartbeat. Ответы буферизуются, чтобы не отправлять
частичный JSON после дедлайна. Закрывшему соединение клиенту доставить `504`
невозможно, но его контекст всё равно отменяет работу.

Воспроизводимая проверка с задержкой PostgreSQL без искусственного `Sleep`
в коде приведена в [корневом README](../README.md#проверка-таймаута-и-остановки-heartbeat).

## Автоматические проверки

Из каталога `gateway`:

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

API-тесты внедряют тестовый `ledger.Service`; PostgreSQL, Redis и сброс
глобального состояния им не нужны. Проверяются DTO, передача контекста,
коды ответов, валидация дат, формат отчёта, таймаут и ошибки отмены контекста.
