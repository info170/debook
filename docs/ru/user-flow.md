# Пользовательский путь бронирования

Ниже описан текущий локальный сценарий: пользователь сначала находит свободный слот, затем бронирует его по `slot_id` и проверяет итоговый статус.

## 0. Подготовка слотов

До пользовательского сценария в БД должны существовать записи `slots` со статусом `AVAILABLE`. Для локальной подготовки используется сидер:

```bash
docker compose run --rm --entrypoint seed-slots api
```

Сидер создаёт одноминутные слоты и удаляет существующий локальный набор бронирований и слотов.

## 1. Поиск свободных слотов

Запрос:

```http
GET /api/v1/slots?region_id=eu-1&resource_id=room-1&status=AVAILABLE&from=2026-01-01T10:00:00Z&to=2026-01-01T12:00:00Z
```

`region_id` и `status` обязательны. Время задаёт включённый диапазон: возвращаются слоты, для которых `starts_at >= from` и `ends_at <= to`. Несколько статусов передаются через запятую и означают альтернативные значения, например `status=RESERVED,BOOKED`.

Сначала API проверяет Redis. При cache miss выполняется запрос PostgreSQL к `slots`, после чего результат сохраняется в Redis на 15 секунд. Этот этап ничего не изменяет в PostgreSQL.

Ответ содержит `slot_id`, который нужно использовать при бронировании.

## 2. Создание брони

Запрос:

```http
POST /api/v1/bookings
Idempotency-Key: booking-001
Content-Type: application/json
```

```json
{
  "slot_id": "...",
  "tenant_id": "company-acme",
  "customer_ref": "user-42",
  "wallet_address": "0x..."
}
```

Booking Service выполняет одну транзакцию PostgreSQL:

1. Проверяет `idempotency_keys` по `(tenant_id, Idempotency-Key)`.
2. Если ключ уже использовался с тем же хэшем запроса, возвращает сохранённый результат без новых записей.
3. Блокирует строку слота через `SELECT ... FOR UPDATE`.
4. Проверяет `slots.status = AVAILABLE`.
5. Обновляет слот: `AVAILABLE → RESERVED` и увеличивает `version`.
6. Создаёт `bookings` со статусом `PENDING_CONFIRMATION`.
7. Создаёт `outbox_events` с типом `BookingReserved`.
8. Сохраняет ответ в `idempotency_keys`.
9. Выполняет `COMMIT`.

API возвращает `202 Accepted` и `booking_id`. При занятом слоте возвращается `409 Conflict`; при отсутствии слота — `404 Not Found`.

## 3. Фоновая обработка

Outbox relay выбирает `outbox_events` с `published_at IS NULL`, публикует событие в Kafka и после успешной публикации заполняет `published_at`.

Workflow worker получает `BookingReserved`, создаёт или обновляет `booking_steps` для шага workflow, а затем в текущем MVP:

- переводит шаг в `SUCCEEDED`;
- переводит `bookings.status` в `CONFIRMED`.

В полном сценарии на этом месте будут выполняться payment, inventory, loyalty и blockchain шаги с retry и компенсациями. При ошибке бронь должна перейти в `FAILED` или `CANCELLED`, а слот — обратно в `AVAILABLE`.

### Blockchain proof

После `POST /api/v1/bookings` outbox передаёт `BookingReserved` в Kafka. Workflow вычисляет SHA-256 от `booking_id`, сохраняет запись `blockchain_proofs` со статусом `PENDING`, вызывает `createBookingProof` в Autheo от backend-minter и после подтверждения транзакции переводит proof в `CONFIRMED`, фиксирует `tx_hash` и переводит бронь в `CONFIRMED`. Проверка выполняется через `GET /api/v1/bookings/{booking_id}/ownership?wallet_address=0x...`, который вызывает read-only метод `isBookingOwned`.

## 4. Проверка статуса

Запрос:

```http
GET /api/v1/bookings/{booking_id}
```

API читает актуальную запись из `bookings`. Пользователь может получить `PENDING_CONFIRMATION`, `CONFIRMED`, `CANCELLED` или `FAILED`.

## 5. Отмена

Запрос:

```http
POST /api/v1/bookings/{booking_id}/cancel
```

В транзакции API блокирует активную бронь и:

1. меняет `bookings.status` на `CANCELLED`;
2. меняет связанный `slots.status` на `AVAILABLE`;
3. увеличивает `slots.version`;
4. выполняет `COMMIT`.

## Идемпотентность и целостность

Повтор с тем же `tenant_id`, `Idempotency-Key` и телом запроса возвращает прежний ответ. Повтор с тем же ключом и другим телом возвращает конфликт. Redis используется для ускорения чтения, а право занять слот определяется только PostgreSQL.
