# Ledger

Ledger хранит бюджеты и расходы в PostgreSQL, кеширует списки бюджетов в Redis
на 20 секунд, а сводные отчёты — на 30 секунд. После изменения бюджета кеш
списка инвалидируется.

## Структура

- `api.go` — доступные Gateway типы, доменные ошибки и интерфейс `Service`.
- `app.go` — публичная фабрика `ledger.New(ctx)`.
- `internal/domain` — сущности, валидация и контракты доступа к данным.
- `internal/service` — один сервис с методами в отдельных файлах для бюджетов,
  транзакций и отчётов; зависит от доменных интерфейсов, а не PostgreSQL/Redis.
- `internal/repository/pg` — реализация SQL-операций без бизнес-правил и HTTP.
- `internal/repository/cached` — Redis-декораторы бюджетов и отчётов.
- `internal/repository/memory` — хранилище для изолированных тестов.
- `internal/app` — чтение окружения, подключения и сборка зависимостей.
- `cmd` — необязательный отдельный запуск Ledger; `migrations` — миграции Goose.

Gateway импортирует только публичный пакет `ledger`: `internal`-пакеты другого
Go-модуля ему недоступны. Публичные сущности и интерфейс предоставлены через
псевдонимы типов, поэтому реализации остаются внутри Ledger.

## Фабрика сервиса

```go
func New(ctx context.Context) (Service, func() error, error)
```

Фабрика проверяет PostgreSQL через `PingContext(ctx)` и Redis через `Ping(ctx)`,
создаёт репозитории и возвращает готовый сервис. После успешного вызова внешний
слой обязан вызвать `closeFn`, обычно через `defer closeFn()`. Функция закрывает
оба ресурса, возвращает ошибки закрытия и допускает повторный вызов.

При ошибке инициализации открытые ресурсы закрываются, возвращаются `nil`, `nil`
и ошибка. Миграции не запускаются. Глобальных клиентов и автоматического
перехода на память при ошибке подключения нет.

Валидация, загрузка бюджетов из JSON и правило
`сумма расходов по категории + новый расход <= лимит` находятся в сервисе.
Без установленного бюджета расход разрешён. Изменения бюджета и добавления
расходов сериализуются внутри одного экземпляра сервиса; между несколькими
процессами общую атомарность эта блокировка не обеспечивает.

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

Для Redis значения по умолчанию: `localhost:6379`, БД `0`, пустой пароль.
Пустое, отрицательное или некорректное значение `REDIS_DB` заменяется на `0`.

## Миграции

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
goose -dir ./migrations postgres "$DATABASE_URL" up
goose -dir ./migrations postgres "$DATABASE_URL" down
```

Миграции нужно применять до первого запуска Ledger и после каждого изменения
схемы.

Фабрика `ledger.New` не создаёт таблицы и не применяет миграции.

## Запуск

```bash
go run ./cmd
```

При успешном запуске Ledger логирует подключения к PostgreSQL и Redis и ждёт
SIGINT/SIGTERM. На подключение выделено 10 секунд; ресурсы закрываются при
завершении. Собственного HTTP-сервера у этой команды нет.

Для работы API отдельно запускать эту команду не нужно: Gateway сам вызывает
фабрику и использует Ledger как Go-библиотеку в своём процессе.

## Проверка

Из каталога `ledger`:

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

Реальные PostgreSQL и Redis для этих тестов не нужны: используются in-memory
репозитории, тестовые реализации интерфейсов, `sqlmock` и `miniredis`.
Сценарии cURL с настоящими зависимостями приведены в [корневом README](../README.md#проверка-сценария-задания).
