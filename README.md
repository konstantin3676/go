# Проект Go

Репозиторий содержит два Go-модуля:

- `gateway` — HTTP-шлюз для учёта расходов;
- `ledger` — сервис учёта расходов (доменная логика).

## Структура и слои

```text
ledger/
├── api.go                     # публичные типы, ошибки и интерфейс Service
├── app.go                     # публичная фабрика ledger.New(ctx)
├── cmd/                       # отдельный запуск проверки подключений Ledger
├── internal/
│   ├── domain/                # сущности, Validate и контракты репозиториев
│   ├── service/               # бизнес-правила бюджетов, транзакций и отчётов
│   ├── repository/
│   │   ├── pg/                # PostgreSQL и SQL-запросы
│   │   ├── cached/            # Redis-декораторы репозиториев
│   │   └── memory/            # изолированное хранилище для тестов
│   └── app/                   # окружение, подключения и сборка зависимостей
└── migrations/                # миграции Goose, запускаемые отдельно
gateway/
├── main.go                    # получение сервиса и запуск HTTP-сервера
└── internal/api/              # обработчики, DTO, ответы и middleware
```

Направление зависимостей:

- домен не зависит от SQL, Redis или HTTP;
- сервис использует только доменные сущности и интерфейсы репозиториев;
- PostgreSQL-репозитории реализуют эти интерфейсы, Redis-декораторы добавляют кеш;
- слой `internal/app` знает о реализациях и собирает сервис;
- Gateway импортирует публичный пакет `ledger`, но не SQL-драйверы и репозитории.

Gateway и Ledger — разные Go-модули, поэтому Gateway не импортирует
`ledger/internal/*`. Корневой пакет `ledger` предоставляет псевдонимы доменных
типов, ошибок и интерфейса сервиса, не раскрывая реализации хранения.

При добавлении транзакции сервис сначала вызывает `Validate()`, затем получает
бюджет и сумму расходов по категории. Равенство лимиту разрешено, превышение
возвращает `ErrBudgetExceeded` без сохранения записи. Если бюджет не задан,
расход разрешён. Репозитории не выполняют эти бизнес-проверки.

### Инициализация в Gateway

Фрагмент функции `run` из `gateway/main.go`:

```go
startupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
svc, closeFn, err := ledger.New(startupCtx)
cancel()
if err != nil {
    return fmt.Errorf("initialize Ledger: %w", err)
}
defer closeFn()

handler := api.LoggingMiddleware(api.NewHandler(svc).Routes())
```

Фабрика читает окружение, создаёт PostgreSQL и Redis, проверяет их через
`PingContext(ctx)` / `Ping(ctx)` и возвращает `(ledger.Service, func() error, error)`.
При частичной ошибке уже открытые ресурсы закрываются; Gateway логирует ошибку
и завершается без запуска HTTP-сервера. Миграции фабрика не запускает.
Функцию закрытия можно безопасно вызвать повторно.

Обработчики получают сервис через конструктор и передают ему `r.Context()`.
Ошибки валидации сервиса отображаются в HTTP `400`, превышение бюджета — в `409`,
ошибки хранения — в `500`. DTO и формат ошибок не изменены.

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

Сервер будет доступен на порту `8080`. Для HTTP API достаточно запустить
PostgreSQL, Redis и Gateway: отдельный процесс Ledger не требуется, так как
Gateway использует его как Go-библиотеку.

На инициализацию выделено 10 секунд. При SIGINT/SIGTERM Gateway прекращает
принимать запросы, ждёт завершения текущих до 5 секунд и закрывает ресурсы Ledger.

## Отдельный запуск Ledger (необязательно)

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
Gateway остаются прежними, данные сохраняются при перезапуске. Автоматического
перехода на память при недоступной БД нет: in-memory репозитории нужны только тестам.

## Проверка проектов

Gateway и Ledger являются независимыми Go-модулями, поэтому проверяются отдельно:

```bash
cd gateway
go test ./...
go test -race ./...
go vet ./...
go build ./...

cd ../ledger
go test ./...
go test -race ./...
go vet ./...
go build ./...
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

HTML-отчёт будет сохранён в `cover.html` соответствующего модуля.
Для автоматических тестов PostgreSQL, Redis, Docker и миграции не нужны:

- домен проверяется отдельно;
- тесты сервиса получают новые in-memory репозитории или тестовые реализации
  интерфейсов для проверки ошибок и передачи контекста;
- SQL-запросы проверяются через `sqlmock`, Redis-кеш — через `miniredis`;
- тесты фабрики проверяют сборку, отмену контекста и закрытие ресурсов;
- API-тесты внедряют тестовый `ledger.Service`, без глобального состояния и `Reset()`.

Эти тесты не заменяют интеграционную проверку реального PostgreSQL. Ручной
сценарий с реальными зависимостями приведён ниже.

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

Примеры ответов ниже предполагают новую БД; ID выдаёт PostgreSQL, поэтому
в существующей БД он может отличаться. Для повторяемой проверки без конфликтов
с прежними данными используйте сценарий в конце README.

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

При отсутствии кеша первый запрос считает суммы в PostgreSQL и сохраняет JSON
в Redis. Повторный запрос в пределах TTL получает тот же результат из Redis.
TTL кеша отчёта — 30 секунд; до истечения TTL отчёт может не отражать новые расходы.
Список бюджетов также кешируется на 20 секунд; успешное изменение бюджета
сбрасывает этот кеш.

## Проверка сценария задания

После применения миграций и запуска PostgreSQL, Redis и Gateway выполните
в отдельном терминале. Используется новая категория для каждого запуска;
сценарий добавляет тестовые данные и не очищает существующие таблицы.

```bash
CATEGORY="проверка-слоев-$(date +%s)-$$"

# 201: установить бюджет и 200: прочитать список (заполняет кеш).
curl -i -X POST http://localhost:8080/api/budgets \
  -H 'Content-Type: application/json' \
  -d "{\"category\":\"$CATEGORY\",\"limit\":1000}"
curl -i http://localhost:8080/api/budgets

# 201: обновить тот же бюджет; следующий GET должен показать лимит 500,
# а не закешированный 1000. Запись категории должна остаться одна.
curl -i -X POST http://localhost:8080/api/budgets \
  -H 'Content-Type: application/json' \
  -d "{\"category\":\"$CATEGORY\",\"limit\":500}"
curl -i http://localhost:8080/api/budgets

# 201: расход в пределах бюджета; ответ содержит ID.
curl -i -X POST http://localhost:8080/api/transactions \
  -H 'Content-Type: application/json' \
  -d "{\"amount\":450,\"category\":\"$CATEGORY\",\"description\":\"ланч\",\"date\":\"2025-09-10\"}"

# 409: {"error":"budget exceeded"}; эта транзакция не должна сохраниться.
curl -i -X POST http://localhost:8080/api/transactions \
  -H 'Content-Type: application/json' \
  -d "{\"amount\":100,\"category\":\"$CATEGORY\",\"date\":\"2025-09-11\"}"

# 201: точное достижение лимита разрешено.
curl -i -X POST http://localhost:8080/api/transactions \
  -H 'Content-Type: application/json' \
  -d "{\"amount\":50,\"category\":\"$CATEGORY\",\"date\":\"2025-09-11\"}"

# 201: отдельная категория без бюджета также разрешена.
curl -i -X POST http://localhost:8080/api/transactions \
  -H 'Content-Type: application/json' \
  -d "{\"amount\":150,\"category\":\"$CATEGORY-без-бюджета\",\"date\":\"2025-09-12\"}"

# 400: {"error":"transaction amount must be positive"}.
curl -i -X POST http://localhost:8080/api/transactions \
  -H 'Content-Type: application/json' -d '{"amount":-100}'

# 400: {"error":"invalid request body"}.
curl -i -X POST http://localhost:8080/api/budgets \
  -H 'Content-Type: application/json' -d '{"category":'

# 200: у $CATEGORY две транзакции с суммой 500; отклонённого расхода нет.
curl -i http://localhost:8080/api/transactions

# 200: сумма для $CATEGORY — 500, для категории без бюджета — 150.
curl -i 'http://localhost:8080/api/reports/summary?from=2025-09-01&to=2025-09-30'
curl -i 'http://localhost:8080/api/reports/summary?from=2025-09-01&to=2025-09-30'
```

Если отчёт за этот период был закеширован раньше, дождитесь истечения его TTL
(30 секунд) перед проверкой новых сумм. Затем остановите Gateway через Ctrl+C,
снова выполните `go run .` из каталога `gateway` и повторите GET-запросы:
бюджет, ID и транзакции должны сохраниться. Новая категория не требует сброса
данных или `TRUNCATE`.

Дополнительно проверьте ошибки отчёта: отсутствие `from` или `to` возвращает
`400` с `{"error":"from and to query parameters are required"}`, неверная дата
или обратный диапазон — `400` с
`{"error":"from and to must be valid dates with from <= to"}`.
