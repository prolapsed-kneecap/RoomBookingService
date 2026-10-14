# Room Booking Service

RESTful сервис бронирования переговорных комнат на Go.

## Стек технологий

- **Язык**: Go 1.22+
- **Маршрутизация**: стандартная библиотека `net/http` (`http.ServeMux` с поддержкой методов и параметров путей)
- **База данных**: PostgreSQL 16
- **Драйвер БД**: `github.com/lib/pq`
- **Аутентификация**: JWT (`golang-jwt/jwt/v5`), хеширование паролей (`golang.org/x/crypto/bcrypt`)
- **Контейнеризация**: Docker Compose

## Архитектура проекта

```text
.
├── cmd/
│   └── server/
│       └── main.go          # Точка входа, запуск сервера, graceful shutdown
├── internal/
│   ├── auth/
│   │   └── auth.go          # Хеширование bcrypt, генерация и парсинг JWT
│   ├── config/
│   │   └── config.go        # Парсинг YAML-конфигурации
│   ├── domain/
│   │   └── domain.go        # Сущности (User, Room, Booking) и кастомные ошибки
│   ├── handler/
│   │   ├── handler.go       # HTTP хендлеры и маппинг ошибок в статус-коды
│   │   └── handler_test.go  # Тесты HTTP-эндпоинтов (httptest)
│   ├── middleware/
│   │   └── middleware.go    # Auth middleware, логирование запросов, recover
│   ├── router/
│   │   └── router.go        # Регистрация маршрутов на базе http.ServeMux
│   ├── service/
│   │   ├── service.go       # Бизнес-логика, валидация интервалов времени
│   │   └── service_test.go  # Юнит-тесты бизнес-логики
│   └── store/
│       └── store.go         # PostgreSQL репозиторий, автомиграция, транзакции
├── config.yaml              # Файл конфигурации сервиса
├── docker-compose.yml       # Контейнер PostgreSQL
├── go.mod
└── README.md
```

## Защита от Double Booking

Защита от состояния гонки и повторного бронирования одной переговорной реализована на уровне транзакции БД:
1. Выполняется эксклюзивная блокировка строки переговорной комнаты:
   ```sql
   SELECT id FROM rooms WHERE id = $1 FOR UPDATE
   ```
2. Проверяется наличие пересечений по интервалам времени:
   ```sql
   SELECT COUNT(*) FROM bookings
   WHERE room_id = $1
     AND start_time < $new_end
     AND end_time > $new_start
   ```
3. При обнаружении коллизии транзакция откатывается и возвращается ошибка `ErrConflict` (HTTP 409 Conflict). Если коллизий нет — бронь фиксируется.

## Запуск сервиса

### 1. Запуск PostgreSQL

```bash
docker compose up -d
```

### 2. Запуск приложения

```bash
go run ./cmd/server
```

Сервер запустится на порту `8080`. При старте автоматически выполняются миграции таблиц `users`, `rooms` и `bookings`.

## API Спецификация

### Аутентификация

- `POST /api/auth/register` — регистрация пользователя
  ```json
  {
    "email": "user@example.com",
    "password": "secretpassword"
  }
  ```
- `POST /api/auth/login` — получение JWT токена
  ```json
  {
    "email": "user@example.com",
    "password": "secretpassword"
  }
  ```
  Ответ:
  ```json
  {
    "token": "<JWT_TOKEN>"
  }
  ```

### Переговорные комнаты

- `GET /api/rooms` — список всех комнат (публичный)
- `POST /api/rooms` — создание комнаты (требуется заголовок `Authorization: Bearer <token>`)
  ```json
  {
    "name": "Meeting Room A",
    "capacity": 10
  }
  ```
- `GET /api/rooms/{id}/schedule?date=YYYY-MM-DD` — расписание занятости комнаты на выбранную дату

### Бронирования

- `POST /api/bookings` — бронирование слота (требуется `Authorization: Bearer <token>`)
  ```json
  {
    "room_id": 1,
    "start_time": "2026-10-15T10:00:00Z",
    "end_time": "2026-10-15T11:00:00Z"
  }
  ```
  *При пересечении с существующей бронью возвращается `409 Conflict`.*
- `GET /api/user/bookings` — список всех броней текущего пользователя (требуется `Authorization: Bearer <token>`)
- `DELETE /api/bookings/{id}` — отмена своей брони (требуется `Authorization: Bearer <token>`)

## Запуск тестов

```bash
go test -v ./...
```
