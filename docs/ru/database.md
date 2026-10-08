# Структура базы данных

Источник схемы — [`migrations/001_initial.sql`](../../migrations/001_initial.sql). PostgreSQL является источником истины для слотов, бронирований, идемпотентности, шагов Saga и outbox-событий. Все даты и время хранятся в UTC.

## `slots` — временные слоты

| Поле | Тип | Обяз. | Назначение |
|---|---|---:|---|
| `id` | UUID | да | Уникальный идентификатор слота. |
| `region_id` | TEXT | да | Регион и ключ маршрутизации к shard. |
| `resource_id` | TEXT | да | Ресурс, которому принадлежит слот. |
| `starts_at` | TIMESTAMPTZ | да | Начало слота. |
| `ends_at` | TIMESTAMPTZ | да | Окончание; должно быть позже `starts_at`. |
| `status` | TEXT | да | `AVAILABLE`, `RESERVED`, `BOOKED` или `CANCELLED`. |
| `version` | BIGINT | да | Версия строки для контроля конкурентных изменений. |
| `created_at` / `updated_at` | TIMESTAMPTZ | да | Время создания и последнего изменения. |

Индексы поддерживают поиск по региону, ресурсу и диапазону времени. Отдельный partial index ускоряет поиск доступных слотов.

## `bookings` — бронирования

| Поле | Тип | Обяз. | Назначение |
|---|---|---:|---|
| `id` | UUID | да | Идентификатор брони, возвращаемый API. |
| `slot_id` | UUID | да | Ссылка на `slots.id`. |
| `tenant_id` | TEXT | да | Организация или клиентская область, в которой создаётся бронь. |
| `customer_ref` | TEXT | да | Конкретный пользователь внутри `tenant_id`; внешний псевдоним без PII. |
| `status` | TEXT | да | `PENDING_CONFIRMATION`, `CONFIRMED`, `CANCELLED` или `FAILED`. |
| `idempotency_key` | TEXT | да | Ключ повторяемого write-запроса. |
| `request_hash` | TEXT | да | Хэш тела запроса для проверки повторов с тем же ключом. |
| `expires_at` | TIMESTAMPTZ | нет | Срок действия временной резервации. |
| `created_at` / `updated_at` | TIMESTAMPTZ | да | Время создания и последнего изменения. |

`tenant_id` отвечает за организацию, а `customer_ref` — за пользователя внутри неё. Например: `company-acme` + `user-42`.

Уникальность `(tenant_id, idempotency_key)` предотвращает дубли. Partial unique index разрешает максимум одну активную бронь (`PENDING_CONFIRMATION` или `CONFIRMED`) на слот.

## `booking_steps` — шаги Saga

| Поле | Тип | Обяз. | Назначение |
|---|---|---:|---|
| `booking_id` | UUID | да | Ссылка на бронь; удаление брони удаляет шаги. |
| `step` | TEXT | да | Имя операции: payment, inventory, loyalty, blockchain и т. п. |
| `status` | TEXT | да | `PENDING`, `RUNNING`, `SUCCEEDED`, `FAILED` или `COMPENSATED`. |
| `attempt` | INTEGER | да | Число попыток выполнения. |
| `last_error` | TEXT | нет | Последняя диагностическая ошибка без секретов и PII. |
| `updated_at` | TIMESTAMPTZ | да | Время последнего изменения шага. |

Первичный ключ — `(booking_id, step)`, поэтому один шаг не дублируется.

## `idempotency_keys` — результаты повторяемых запросов

| Поле | Тип | Обяз. | Назначение |
|---|---|---:|---|
| `tenant_id` | TEXT | да | Организация и область уникальности ключа. |
| `key` | TEXT | да | Значение заголовка `Idempotency-Key`. |
| `request_hash` | TEXT | да | Хэш исходного запроса. |
| `response_status` | INTEGER | нет | Сохранённый HTTP-статус ответа. |
| `response_payload` | JSONB | нет | Сохранённое тело ответа для повтора. |
| `expires_at` | TIMESTAMPTZ | да | Когда запись можно удалить по retention policy. |
| `created_at` | TIMESTAMPTZ | да | Время первого запроса. |

Первичный ключ — `(tenant_id, key)`. Запрос с тем же ключом, но другим `request_hash`, должен завершаться ошибкой конфликта.

## `outbox_events` — события для публикации

| Поле | Тип | Обяз. | Назначение |
|---|---|---:|---|
| `id` | UUID | да | Уникальный `event_id`. |
| `aggregate_id` | UUID | да | Идентификатор брони или другого агрегата. |
| `type` | TEXT | да | Тип события, например `BookingReserved`. |
| `schema_version` | INTEGER | да | Версия формата payload. |
| `payload` | JSONB | да | Тело события без PII, не предназначенное для прямого API-ответа. |
| `correlation_id` | TEXT | нет | Связь с исходным пользовательским запросом. |
| `causation_id` | TEXT | нет | Событие или команда, вызвавшие это событие. |
| `published_at` | TIMESTAMPTZ | нет | Время успешной публикации в Kafka; `NULL` означает pending. |
| `created_at` | TIMESTAMPTZ | да | Время записи события в той же транзакции, что и бронь. |

Outbox relay выбирает записи с `published_at IS NULL`, публикует их идемпотентно и затем отмечает время публикации. Индекс `idx_outbox_unpublished` ускоряет выборку.

## Правила целостности

- Резервирование слота и запись брони выполняются в одной транзакции PostgreSQL.
- Redis, Kafka и ClickHouse не являются источниками истины для конкурентного бронирования.
- Изменение схемы выполняется новыми версионными миграциями; существующие миграции не редактируются.
- PII не помещается в `customer_ref`, outbox payload или blockchain proof; используется внешний псевдоним.
