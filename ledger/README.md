# Ledger

Ledger хранит бюджеты и расходы в PostgreSQL, а сводные отчёты кеширует в
Redis на 30 секунд.

## Запуск зависимостей

```bash
docker run --name cashcraft-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=cashapp \
  -p 5432:5432 -d postgres:16-alpine

docker run --name cashcraft-redis -p 6379:6379 -d redis:7-alpine
```

Переменные окружения:

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/cashapp?sslmode=disable'
export REDIS_ADDR=localhost:6379
export REDIS_DB=0
export REDIS_PASSWORD=''
```

Вместо `DATABASE_URL` можно задать `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASS` и
`DB_NAME`. Значения по умолчанию: `localhost`, `5432`, `postgres`, `postgres` и
`cashapp`.

## Миграции

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
goose -dir ./migrations postgres "$DATABASE_URL" up
goose -dir ./migrations postgres "$DATABASE_URL" down
```

Миграции нужно применять до первого запуска Ledger и после каждого изменения
схемы.

## Запуск

```bash
go run ./cmd
```

При успешном запуске Ledger логирует подключения к PostgreSQL и Redis. Gateway
обращается к тем же HTTP endpoint-ам, но Ledger теперь использует постоянное
хранилище PostgreSQL вместо памяти процесса.
