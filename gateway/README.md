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

handler := api.LoggingMiddleware(api.NewHandler(svc).Routes())
```

Этот код находится в функции `run`; ошибка возвращается в `main`, логируется,
и процесс завершается. На инициализацию выделено 10 секунд.
Конфигурация, `sql.Open`, PostgreSQL, Redis и сборка репозиториев скрыты в Ledger.
Gateway не импортирует драйверы БД и пакеты репозиториев и не запускает миграции.
Отдельный процесс Ledger для работы Gateway не требуется.

Обработчики сохраняют прежние DTO и вызывают методы сервиса с `r.Context()`.
Валидацию сущностей и бюджетное правило выполняет сервис, HTTP-слой отвечает
за JSON, параметры запроса и отображение ошибок: `400` — валидация,
`409` — превышение бюджета, `500` — внутренняя ошибка.
Формат ошибки: `{"error":"описание ошибки"}`.

## Запуск

Перед запуском примените миграции из корня репозитория:

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/cashapp?sslmode=disable'
export REDIS_ADDR=localhost:6379
export REDIS_DB=0
export REDIS_PASSWORD=''

goose -dir ./ledger/migrations postgres "$DATABASE_URL" up

cd gateway
go run .
```

Gateway доступен на `http://localhost:8080`.

PostgreSQL и Redis должны быть доступны до запуска. При ошибке подключения
сервер не стартует. SIGINT/SIGTERM запускает остановку HTTP-сервера с ожиданием
текущих запросов до 5 секунд, после чего освобождаются ресурсы Ledger.

## Проверка API

Пример рассчитан на категорию без прежних расходов; повторяемый сценарий
с уникальной категорией и проверкой перезапуска описан в
[корневом README](../README.md#проверка-сценария-задания).

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
коды ответов и прежний формат ошибок.
