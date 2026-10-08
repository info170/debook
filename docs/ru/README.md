# Система бронирования слотов

Документация проекта ведётся на русском и английском языках.

- [System Design](system-design.md)


- [Database structure](database.md)
- [OpenAPI / Swagger](../openapi.yaml)


## Swagger / OpenAPI

Спецификация API хранится в [openapi.yaml](../openapi.yaml). При запущенном Compose Swagger UI доступен по адресу [http://localhost:8081](http://localhost:8081).

```bash
docker compose up -d swagger-ui
```

Swagger описывает health/readiness, поиск слотов, создание, получение и отмену бронирования. API работает на [http://localhost:8080](http://localhost:8080).

## Сидер слотов

Сидер очищает текущие слоты и связанные с ними бронирования, затем создаёт по одному доступному слоту на каждую минуту. Значения по умолчанию — 365 дней, то есть 525600 слотов.

```bash
docker compose run --rm --entrypoint seed-slots api
```

Параметры можно изменить:

```bash
docker compose run --rm --entrypoint seed-slots api \
  -region eu-1 \
  -resource room-1 \
  -days 365 \
  -start 2026-01-01T00:00:00Z
```

Сидер выполняет `TRUNCATE ... CASCADE`, поэтому удаляет существующие брони, Saga-состояния, idempotency-записи, outbox-события и слоты. Запускайте его только для подготовки или полного пересоздания локальных данных.



## Поиск слотов

Используйте `GET /api/v1/slots`. Параметры `region_id` и `status` обязательны. `resource_id`, `from` и `to` необязательны. В `status` можно передать несколько значений через запятую, например `status=RESERVED,BOOKED`; они означают альтернативные статусы (`OR`), а остальные фильтры применяются совместно (`AND`).

```text
GET /api/v1/slots?region_id=eu-1&resource_id=room-1&status=AVAILABLE&from=2026-01-01T00:00:00Z&to=2026-01-01T02:00:00Z
```
- [Путь пользователя при бронировании](user-flow.md)
- [Настройки Autheo Testnet / Mainnet](blockchain-config.md)
- [Blockchain deployer](../../blockchain-deployer/README.md)
- [Наблюдаемость](observability.md)
- [Безопасность](security.md)
