# Gateway

Gateway предоставляет HTTP API для бюджетов, расходов и сводного отчёта.
Данные сохраняются Ledger в PostgreSQL, а отчёты кешируются Ledger в Redis.

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

## Проверка API

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

Превышающая лимит транзакция возвращает `409 Conflict`. Повторный запрос
отчёта в течение 30 секунд получает результат из Redis.
