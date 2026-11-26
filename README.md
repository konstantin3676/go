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

handler := api.LoggingMiddleware(api.TimeoutMiddleware(timeout, api.NewHandler(svc).Routes()))
```

Фабрика читает окружение, создаёт PostgreSQL и Redis, проверяет их через
`PingContext(ctx)` / `Ping(ctx)` и возвращает `(ledger.Service, func() error, error)`.
При частичной ошибке уже открытые ресурсы закрываются; Gateway логирует ошибку
и завершается без запуска HTTP-сервера. Миграции фабрика не запускает.
Функцию закрытия можно безопасно вызвать повторно.

Обработчики получают сервис через конструктор и передают ему `r.Context()`.
Ошибки валидации сервиса отображаются в HTTP `400`, превышение бюджета — в `409`,
ошибки хранения — в `500`. DTO бюджетов и транзакций и формат ошибок не изменены.

Отмена операции и истечение контекста запроса возвращают `504` в едином формате
JSON. `timeout` читается из `REQUEST_TIMEOUT`, по умолчанию — 2 секунды.

## Требования

- Go 1.25 или новее
- PostgreSQL 14 или новее
- Redis 7 или новее
- Goose для миграций

## Запуск через Docker Compose

Понадобятся Docker с BuildKit и Docker Compose 2.17 или новее (для
`additional_contexts`). Все команды в этом разделе выполняются из корня
репозитория. В Compose запускаются PostgreSQL, Redis и Gateway; Ledger
встроен в Gateway как Go-библиотека и отдельным контейнером не запускается.

1. Запустите PostgreSQL и Redis и дождитесь статуса `healthy`:

   ```bash
   docker compose up -d --wait db redis
   docker compose ps
   ```

   Если порт `5432` уже занят, перед запуском задайте `export DB_HOST_PORT=15432`.
   Это меняет только порт на хосте; внутри сети Compose PostgreSQL по-прежнему
   доступен как `db:5432`.

2. Примените миграции Goose **с хоста, до запуска Gateway**:

   ```bash
   export DATABASE_URL="postgres://postgres:postgres@localhost:${DB_HOST_PORT:-5432}/cashapp?sslmode=disable"
   goose -dir ./ledger/migrations postgres "$DATABASE_URL" up
   ```

   Если Goose не установлен: `go install github.com/pressly/goose/v3/cmd/goose@latest`.

3. Соберите и запустите Gateway, затем проверьте healthcheck и `/ping`:

   ```bash
   docker compose up -d --build --wait gateway
   docker compose ps
   docker compose exec -T gateway wget -qO- http://localhost:8080/ping
   curl -fsS http://localhost:8080/ping
   ```

   В `docker compose ps` у Gateway должен быть статус `healthy`; обе команды
   проверки должны вывести `pong`.

Учебные настройки в `docker-compose.yml`: `POSTGRES_USER=postgres`,
`POSTGRES_PASSWORD=postgres`, `POSTGRES_DB=cashapp`. Gateway получает
`DATABASE_URL=postgres://postgres:postgres@db:5432/cashapp?sslmode=disable`
и `REDIS_ADDR=redis:6379`. Эти значения заданы в Compose: переменные
`DB_USER`, `DB_PASS` и `DB_NAME` с хоста их не переопределяют. Ledger читает
`DB_*` только при отсутствии `DATABASE_URL`. PostgreSQL применяет `POSTGRES_*`
при инициализации пустого тома; изменение этих переменных не меняет учётные
данные или имя базы в уже существующем томе.

### Быстрая проверка API

```bash
CATEGORY="compose-check-$(date +%s)-$$"
BASE_URL=http://localhost:8080

# 201: создать бюджет; 200: прочитать список бюджетов.
curl -fsi "$BASE_URL/api/budgets" -H 'Content-Type: application/json' \
  -d "{\"category\":\"$CATEGORY\",\"limit\":1000}"
curl -fsi "$BASE_URL/api/budgets"

# 201: создать транзакцию; 200: импортировать ещё одну и прочитать обе.
curl -fsi "$BASE_URL/api/transactions" -H 'Content-Type: application/json' \
  -d "{\"amount\":450,\"category\":\"$CATEGORY\",\"date\":\"2025-09-10\"}"
curl -fsi "$BASE_URL/api/transactions/bulk" -H 'Content-Type: application/json' \
  -d "[{\"amount\":50,\"category\":\"$CATEGORY\",\"date\":\"2025-09-11\"}]"
curl -fsi "$BASE_URL/api/transactions"

# 200: сумма для новой категории — 500.
curl -fsi "$BASE_URL/api/reports/summary?from=2025-09-10&to=2025-09-11"
```

Если отчёт за эти даты был недавно запрошен, дождитесь истечения TTL кеша
(30 секунд), чтобы увидеть новые записи.

### Сборка образов и отдельный запуск Gateway

Оставьте `db` и `redis` запущенными, а Gateway в Compose остановите, чтобы
освободить порт `8080`. Сеть `cashcraft_default` создаётся при запуске Compose
с именем проекта по умолчанию (`cashcraft`).

```bash
docker build --build-context ledger_src=./ledger -t cashcraft-gateway:local ./gateway
docker build -t cashcraft-ledger:local ./ledger

docker compose stop gateway
docker run --rm -d --name cashcraft-gateway-local --network cashcraft_default \
  -p 8080:8080 \
  -e 'DATABASE_URL=postgres://postgres:postgres@db:5432/cashapp?sslmode=disable' \
  -e REDIS_ADDR=redis:6379 \
  cashcraft-gateway:local
curl -fsS http://localhost:8080/ping

docker stop cashcraft-gateway-local
docker compose down  # Именованный том с данными PostgreSQL сохраняется.
```

## PostgreSQL и Redis для локального запуска без Compose

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

## Локальный запуск Gateway

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
- параллельный запуск расчётов, отмена работников и завершение heartbeat
  проверяются отдельными тестами; `-race` проверяет доступ к результатам;
- тесты фабрики проверяют сборку, отмену контекста и закрытие ресурсов;
- API-тесты внедряют тестовый `ledger.Service`, без глобального состояния и `Reset()`.

Эти тесты не заменяют интеграционную проверку реального PostgreSQL. Ручной
сценарий с реальными зависимостями приведён ниже.

## Таймауты запросов

Ко всем API-ручкам Gateway применяется middleware с `context.WithTimeout`.
По умолчанию на обработку запроса выделяется **2 секунды**. Настройка читается
один раз при запуске из `REQUEST_TIMEOUT` в формате Go duration:

```bash
cd gateway
REQUEST_TIMEOUT=2s go run .
# Для проверки отмены: остановите предыдущий процесс и запустите:
REQUEST_TIMEOUT=50ms go run .
```

Если переменная отсутствует или пуста, используется `2s`. Некорректное,
нулевое или отрицательное значение не позволяет серверу запуститься.

При превышении таймаута или ошибке отмены контекста API возвращает
**504 Gateway Timeout**, `Content-Type: application/json; charset=utf-8` и:

```json
{ "error": "request timed out or canceled" }
```

Контекст передаётся через `r.WithContext(ctx)` в обработчики и далее в Ledger,
PostgreSQL (`QueryContext`, `QueryRowContext`, `ExecContext`) и Redis. При отмене
расчёт прекращается, метод дожидается работников и завершения heartbeat.
Ответ буферизуется до завершения обработчика: после истечения дедлайна клиент
не получит частичный JSON или случайный `200`.

Отмена кооперативная: middleware не убивает Go-горутину принудительно,
обработчики и зависимости должны учитывать контекст. Здесь нет отдельной
горутины HTTP-обработчика, оставляемой работать после ответа `504`. Если клиент
уже закрыл соединение, доставить ему ответ невозможно, но контекст всё равно
отменяется и освобождает ресурсы.

## API Gateway

Ответы `/api/*` возвращаются с заголовком `Content-Type: application/json; charset=utf-8`. Ошибки возвращаются в формате `{"error":"описание ошибки"}`. `GET /ping` возвращает `pong` как обычный текст.

### Эндпоинты

| Метод | Путь                                                 | Описание                       | Успех                                                 |
| ----- | ---------------------------------------------------- | ------------------------------ | ----------------------------------------------------- |
| GET   | `/ping`                                              | Проверка доступности Gateway   | 200 OK                                                |
| POST  | `/api/transactions`                                  | Создать транзакцию             | 201 Created                                           |
| POST  | `/api/transactions/bulk`                             | Импортировать пакет транзакций | 200 OK (в том числе при отклонении отдельных записей) |
| GET   | `/api/transactions`                                  | Список транзакций              | 200 OK                                                |
| POST  | `/api/budgets`                                       | Создать или обновить бюджет    | 201 Created                                           |
| GET   | `/api/budgets`                                       | Список бюджетов                | 200 OK                                                |
| GET   | `/api/reports/summary?from=YYYY-MM-DD&to=YYYY-MM-DD` | Сводный отчёт                  | 200 OK                                                |

### Коды ошибок

| Код | Ситуация                                           |
| --- | -------------------------------------------------- |
| 400 | Ошибка валидации или некорректное тело запроса     |
| 409 | Превышение бюджета                                 |
| 500 | Внутренняя ошибка сервера                          |
| 504 | Таймаут запроса или отмена операции через контекст |

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
{
  "id": 1,
  "amount": 450,
  "category": "еда",
  "description": "ланч",
  "date": "2025-09-10"
}
```

**Пакетный импорт транзакций:**

`POST /api/transactions/bulk` принимает JSON-массив объектов в том же формате,
что и одиночный POST: `amount`, `category`, `description`, `date`. Необязательный
query-параметр `workers` задаёт размер пула (целое число от 1 до 32, по умолчанию 4).
Некорректный JSON, тело не в виде массива объектов или некорректный `workers`
дают `400` без запуска импорта.

```bash
curl -i -X POST 'http://localhost:8080/api/transactions/bulk?workers=4' \
  -H 'Content-Type: application/json' \
  -d '[
    {"amount":120,"category":"книги","description":"учебник","date":"2025-09-10"},
    {"amount":-20,"category":"книги","description":"ошибка суммы","date":"2025-09-10"}
  ]'
```

Ожидаемый ответ на новой БД (`200 OK`):

```json
{
  "accepted": 1,
  "rejected": 1,
  "errors": [{ "index": 1, "error": "transaction amount must be positive" }]
}
```

`accepted` — число сохранённых транзакций, `rejected` — число отклонённых (например,
из-за валидации или превышения бюджета). Индексы в `errors` начинаются с нуля и
отсортированы по возрастанию. Воркеры не гарантируют порядок сохранения: если на
всю категорию бюджета не хватает, заранее неизвестно, какая из записей пройдёт первой.

Пустой пакет с настройками по умолчанию возвращает `200 OK` и
`{"accepted":0,"rejected":0,"errors":[]}`:

```bash
curl -i -X POST 'http://localhost:8080/api/transactions/bulk' \
  -H 'Content-Type: application/json' -d '[]'
```

При таймауте или отмене Gateway возвращает `504` с прежним JSON
`{"error":"request timed out or canceled"}`. Ledger возвращает частичную сводку
вызывающему коду; уже сохранённые транзакции не откатываются. Не повторяйте
весь пакет после `504` без проверки записей: повторный запрос может создать дубли.

**Ошибка валидации (400):**

```bash
curl -X POST http://localhost:8080/api/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":-100,"category":"","description":"","date":""}'
```

Ответ:

```json
{ "error": "transaction amount must be positive" }
```

**Список транзакций:**

```bash
curl http://localhost:8080/api/transactions
```

Ответ (200 OK):

```json
[
  {
    "id": 1,
    "amount": 450,
    "category": "еда",
    "description": "ланч",
    "date": "2025-09-10"
  }
]
```

**Создание бюджета:**

```bash
curl -X POST http://localhost:8080/api/budgets \
  -H "Content-Type: application/json" \
  -d '{"category":"еда","limit":1000}'
```

Ответ (201 Created):

```json
{ "category": "еда", "limit": 1000 }
```

**Ошибка валидации бюджета (400):**

```bash
curl -X POST http://localhost:8080/api/budgets \
  -H "Content-Type: application/json" \
  -d '{"category":"","limit":0}'
```

Ответ:

```json
{ "error": "budget limit must be positive" }
```

**Список бюджетов:**

```bash
curl http://localhost:8080/api/budgets
```

Ответ (200 OK):

```json
[{ "category": "еда", "limit": 1000 }]
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
{ "error": "budget exceeded" }
```

**Сводный отчёт:**

```bash
curl 'http://localhost:8080/api/reports/summary?from=2025-09-01&to=2025-09-30'
curl 'http://localhost:8080/api/reports/summary?from=2025-09-01&to=2025-09-30'
```

`from` и `to` обязательны, имеют формат `YYYY-MM-DD`, `from <= to`.
Обе границы периода включены; одинаковые даты задают отчёт за один день.
Ответ — JSON-объект, где ключом служит категория, а значением — сумма расходов:

```json
{ "еда": 1234.56, "транспорт": 789 }
```

Числа приведены для примера и зависят от сохранённых расходов. Если расходов
за период нет, ответ — `{}`. Отсутствие даты, неверный формат, несуществующая
дата или обратный диапазон возвращают `400` с `{"error":"описание ошибки"}`.
Отмена расчёта возвращает `504`, описанный в разделе «Таймауты запросов».

При отсутствии кеша Ledger получает `DISTINCT` категорий расходов за период.
Для каждой категории отдельная горутина считает сумму в PostgreSQL с тем же
контекстом. `sync.WaitGroup` ожидает завершения; каждый работник записывает
только свой элемент среза, результаты читаются после `Wait()`. Ошибка работника
отменяет остальные вычисления, частичный отчёт не возвращается.

Во время расчёта работает одна heartbeat-горутина с `time.Ticker` на 400 мс.
Она пишет `report calculation in progress`; при завершении или отмене тикер
останавливается и горутина завершается до возврата из метода. На попадании
в кеш работники и heartbeat не запускаются.

Готовый результат сохраняется в Redis. Повторный запрос в пределах TTL получает
тот же результат из Redis. Внутренний формат кеша остаётся массивом записей;
Gateway преобразует его в объект категорий при формировании HTTP-ответа.
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
(30 секунд) перед проверкой новых сумм. Затем выполните `docker compose restart gateway`
из корня репозитория. При локальном запуске вместо этого остановите Gateway через
Ctrl+C и снова выполните `go run .` из каталога `gateway`. Повторите GET-запросы:
бюджет, ID и транзакции должны сохраниться. Новая категория не требует сброса
данных или `TRUNCATE`.

Дополнительно проверьте ошибки отчёта: отсутствие `from` или `to` возвращает
`400` с `{"error":"from and to query parameters are required"}`, неверная дата
или обратный диапазон — `400` с
`{"error":"from and to must be valid dates with from <= to"}`.

### Проверка таймаута и остановки heartbeat

Сценарий изменения `REQUEST_TIMEOUT` ниже рассчитан на [локальный запуск Gateway](#локальный-запуск-gateway):
текущий Compose не передаёт эту переменную с хоста в контейнер Gateway.

1. С таймаутом `2s` выполните запрос отчёта из примера выше: ожидаются `200`
   и суммы по категориям. Затем остановите Gateway и запустите
   `REQUEST_TIMEOUT=50ms go run .` из каталога `gateway`.
2. Для воспроизводимого таймаута нужна задержка: быстрый запрос, особенно из
   Redis, может успеть даже за 1 мс. На **локальной учебной БД**, в отдельном
   терминале откройте psql и временно заблокируйте чтение таблицы:

   ```bash
   docker exec -it cashcraft-postgres psql -U postgres -d cashapp
   ```

   Если Gateway подключён к базе из Compose, откройте `psql` в ней:

   ```bash
   docker compose exec db psql -U postgres -d cashapp
   ```

   ```sql
   BEGIN;
   LOCK TABLE expenses IN ACCESS EXCLUSIVE MODE;
   ```

   После сообщения `LOCK TABLE` оставьте эту сессию открытой. Блокировка не
   меняет данные, но временно задерживает и другие обращения к таблице.

3. Выберите незакешированный диапазон. Например, дождитесь истечения TTL
   отчёта (30 секунд без запросов к нему) и выполните:

   ```bash
   curl -i 'http://localhost:8080/api/reports/summary?from=2025-09-01&to=2025-09-29'
   ```

   Ожидается `504` с `{"error":"request timed out or canceled"}`. В журнале:

   ```text
   report heartbeat stopped; calculation interrupted (ctx.Done): context deadline exceeded
   ```

   При таймауте 50 мс сообщение раз в 400 мс может не успеть появиться — это
   нормально. Чтобы увидеть периодический heartbeat, запустите запрос с `2s`
   и удерживайте блокировку дольше 400 мс.

4. В терминале psql проверьте отсутствие оставшихся ожидающих SQL-запросов:

   ```sql
   SELECT pid, state, wait_event_type, query
   FROM pg_stat_activity
   WHERE datname = current_database()
     AND pid <> pg_backend_pid()
     AND wait_event_type = 'Lock';
   ```

   Если нет других клиентов, результат должен быть пустым. Затем **обязательно**
   освободите блокировку и закройте psql:

   ```sql
   ROLLBACK;
   \q
   ```

5. Повторите запрос без блокировки — ожидается `200`. Остановите Gateway и
   верните разумный таймаут: `REQUEST_TIMEOUT=2s go run .` либо уберите переменную.

Завершение всех работников до возврата и остановка heartbeat дополнительно
проверяются автоматическими тестами, включая `go test -race ./...`.
