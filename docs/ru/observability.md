# Наблюдаемость

Локальный Compose уже включает Prometheus и Jaeger. API публикует `/metrics` с базовыми счётчиками HTTP-запросов и ответов 5xx; Prometheus настроен на их сбор по адресу `api:8080/metrics`. API пишет структурированные JSON-логи с методом, путём, статусом, длительностью и `request_id`.

Для диагностики бронирования используются `booking_id`, Kafka/outbox-логи и таблицы `booking_steps` и `blockchain_proofs`. Проверка состояния контейнеров выполняется через `/healthz` и `/readyz`. Интерфейсы доступны локально: Prometheus — `http://localhost:9090`, Jaeger — `http://localhost:16686`.

В API добавлены таймауты чтения/записи и ограничение тела POST-запроса 64 KiB. Это базовый слой наблюдаемости для локальной среды; для production следует добавить экспорт трассировок OTLP, dashboards и алерты.

API создаёт `request_id`, возвращает его в `X-Request-ID` и записывает в лог. `/readyz` проверяет PostgreSQL и Redis, а `/healthz` только показывает работоспособность процесса.
