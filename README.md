# Проект Go

Репозиторий содержит два Go-модуля:

- `gateway` — HTTP-шлюз для учёта расходов;
- `ledger` — сервис учёта расходов (доменная логика).

## Требования

- Go 1.25 или новее

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

При запуске Ledger загружает бюджеты из `ledger/budgets.json`, создаёт тестовые транзакции и проверяет их суммы относительно лимитов категорий. Запускайте сервис из каталога `ledger`, чтобы файл бюджетов был доступен по ожидаемому пути:

```text
Ledger service started
Transaction rejected: budget exceeded for category "Продукты"
Transactions:
ID: 1, amount: 1250.50, category: Продукты, description: Покупка продуктов, date: 2026-10-01
ID: 2, amount: 350.00, category: Транспорт, description: Поездка на такси, date: 2026-10-02
ID: 3, amount: 2400.00, category: Образование, description: Книга по Go, date: 2026-10-02
```

Лимит категории «Продукты» равен 2000 рублей. Первая транзакция на 1250.50 рубля сохраняется, а следующая транзакция на 1000 рублей отклоняется. Для категории «Образование» бюджет не установлен, поэтому транзакция сохраняется без проверки лимита.

Бюджеты и транзакции находятся в оперативной памяти и удаляются после завершения программы.

## Проверка проектов

Gateway и Ledger являются независимыми Go-модулями, поэтому проверяются отдельно:

```bash
cd gateway
go test ./...

cd ../ledger
go test ./...
```

## API Gateway

Все ответы возвращаются с заголовком `Content-Type: application/json; charset=utf-8`. Ошибки возвращаются в формате `{"error":"описание ошибки"}`.

### Эндпоинты

| Метод | Путь | Описание | Успех |
|-------|------|----------|-------|
| POST | `/api/transactions` | Создать транзакцию | 201 Created |
| GET | `/api/transactions` | Список транзакций | 200 OK |
| POST | `/api/budgets` | Создать или обновить бюджет | 201 Created |
| GET | `/api/budgets` | Список бюджетов | 200 OK |

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