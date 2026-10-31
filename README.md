# Проект Go

Репозиторий содержит два Go-модуля:

- `gateway` — HTTP-шлюз для учёта расходов;
- `ledger` — сервис учёта расходов (доменная логика).

## Требования

- Go 1.25 или новее
- PostgreSQL 14 или новее
- Redis 7 или новее
- Goose для миграций

## PostgreSQL и Redis

Быстрый запуск PostgreSQL и Redis через Docker:

```bash
docker run --name cashcraft-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=cashapp \
  -p 5432:5432 -d postgres:16-alpine

docker run --name cashcraft-redis \
  -p 6379:6379 -d redis:7-alpine
```

Настройка подключения по умолчанию подходит для этих контейнеров:

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/cashapp?sslmode=disable'
export REDIS_ADDR=localhost:6379
export REDIS_DB=0
export REDIS_PASSWORD=''
```

Если `DATABASE_URL` не задана, Ledger собирает DSN из `DB_HOST`, `DB_PORT`,
`DB_USER`, `DB_PASS` и `DB_NAME`.

Установите Goose и примените миграции до запуска сервисов:

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
goose -dir ./ledger/migrations postgres "$DATABASE_URL" up
```

Откат последней миграции:

```bash
goose -dir ./ledger/migrations postgres "$DATABASE_URL" down
```

## Запуск Gateway

Из корня репозитория выполните:

```bash
cd gateway
go run .
```

Сервер будет доступен на порту `8080`.

## Запуск Ledger

Из корня репозитория выполните:

```bash
cd ledger
go run ./cmd
```

Ledger при запуске проверяет подключения к PostgreSQL и Redis и оставляет сервис
запущенным до получения SIGINT/SIGTERM. При успешном старте в журнале появятся
сообщения о подключении к обеим системам.

```text
[... ] Ledger connected to PostgreSQL
[... ] Ledger connected to Redis
[... ] Ledger service started
```

Gateway использует тот же пакет Ledger и те же PostgreSQL/Redis. HTTP-контракты
Gateway остаются прежними, а данные больше не теряются при перезапуске.

## Проверка проектов

Gateway и Ledger являются независимыми Go-модулями, поэтому проверяются отдельно:

```bash
cd gateway
go test ./...

cd ../ledger
go test ./...
```

### Тесты и покрытие

Запустить тесты с построением отчёта о покрытии можно из корня репозитория:

```bash
cd ledger
go test ./... -cover -coverprofile=cover.out
go tool cover -html=cover.out -o cover.html

cd ../gateway
go test ./... -cover -coverprofile=cover.out
go tool cover -html=cover.out -o cover.html
```

HTML-отчёт будет сохранён в `cover.html` соответствующего модуля. После
изменений с PostgreSQL и Redis тесты используют режим без подключенной БД для
быстрых проверок доменной логики; ручная проверка с реальными сервисами описана
ниже.

## API Gateway

Все ответы возвращаются с заголовком `Content-Type: application/json; charset=utf-8`. Ошибки возвращаются в формате `{"error":"описание ошибки"}`.

### Эндпоинты

| Метод | Путь | Описание | Успех |
|-------|------|----------|-------|
| POST | `/api/transactions` | Создать транзакцию | 201 Created |
| GET | `/api/transactions` | Список транзакций | 200 OK |
| POST | `/api/budgets` | Создать или обновить бюджет | 201 Created |
| GET | `/api/budgets` | Список бюджетов | 200 OK |
| GET | `/api/reports/summary?from=YYYY-MM-DD&to=YYYY-MM-DD` | Сводный отчёт | 200 OK |

### Коды ошибок

| Код | Ситуация |
|-----|----------|
| 400 | Ошибка валидации или некорректное тело запроса |
| 409 | Превышение бюджета |
| 500 | Внутренняя ошибка сервера |

### Примеры cURL

**Создание транзакции:**

```bash
curl -X POST http://localhost:8080/api/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":450,"category":"еда","description":"ланч","date":"2025-09-10"}'
```

Ответ (201 Created):
```json
{"id":1,"amount":450,"category":"еда","description":"ланч","date":"2025-09-10"}
```

**Ошибка валидации (400):**

```bash
curl -X POST http://localhost:8080/api/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":-100,"category":"","description":"","date":""}'
```

Ответ:
```json
{"error":"transaction amount must be positive"}
```

**Список транзакций:**

```bash
curl http://localhost:8080/api/transactions
```

Ответ (200 OK):
```json
[{"id":1,"amount":450,"category":"еда","description":"ланч","date":"2025-09-10"}]
```

**Создание бюджета:**

```bash
curl -X POST http://localhost:8080/api/budgets \
  -H "Content-Type: application/json" \
  -d '{"category":"еда","limit":1000}'
```

Ответ (201 Created):
```json
{"category":"еда","limit":1000}
```

**Ошибка валидации бюджета (400):**

```bash
curl -X POST http://localhost:8080/api/budgets \
  -H "Content-Type: application/json" \
  -d '{"category":"","limit":0}'
```

Ответ:
```json
{"error":"budget limit must be positive"}
```

**Список бюджетов:**

```bash
curl http://localhost:8080/api/budgets
```

Ответ (200 OK):
```json
[{"category":"еда","limit":1000}]
```

**Превышение бюджета (409):**

```bash
curl -X POST http://localhost:8080/api/budgets \
  -H "Content-Type: application/json" \
  -d '{"category":"еда","limit":500}'

curl -X POST http://localhost:8080/api/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":600,"category":"еда","description":"обед","date":"2025-09-10"}'
```

Ответ:
```json
{"error":"budget exceeded"}
```

**Сводный отчёт:**

```bash
curl 'http://localhost:8080/api/reports/summary?from=2025-09-01&to=2025-09-30'
curl 'http://localhost:8080/api/reports/summary?from=2025-09-01&to=2025-09-30'
```

Первый запрос считает суммы в PostgreSQL и сохраняет JSON в Redis. Второй
запрос получает тот же результат из Redis. TTL кеша отчёта — 30 секунд.

## Проверка сценария задания

После запуска Ledger и Gateway можно выполнить:

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
```

Последний POST вернёт `409`, а запись не появится в `expenses`. После
перезапуска Gateway данные будут прочитаны из PostgreSQL.
