# Настройки Autheo

Интеграция с Autheo будет использовать Testnet на этапе разработки и переключаться на Mainnet конфигурацией, без изменения кода. Значения окружения задаются в корневом `.env`; файл игнорируется Git. Шаблон переменных — [`.env.example`](../../.env.example).

Текущий Testnet: EVM RPC `https://rpc.testnet.autheo.com`, Cosmos RPC `https://cosmos-rpc.testnet.autheo.com`, REST API `https://rest.testnet.autheo.com`, chain ID `21270`, валюта `THEO` (18 decimals). Faucet: `https://faucet.testnet.autheo.com`. Explorer: `https://evm-explorer.testnet.autheo.com` и `https://cosmos.testnet.autheo.com`. Старый Testnet с chain ID `785` выводится из эксплуатации; его использовать не следует. Перед переключением на Mainnet нужно согласованно изменить `AUTHEO_NETWORK`, `AUTHEO_RPC_URL`, `AUTHEO_CHAIN_ID` и адрес контракта.

```env
AUTHEO_NETWORK=testnet
AUTHEO_RPC_URL=https://rpc.testnet.autheo.com
AUTHEO_COSMOS_RPC_URL=https://cosmos-rpc.testnet.autheo.com
AUTHEO_REST_URL=https://rest.testnet.autheo.com
AUTHEO_FAUCET_URL=https://faucet.testnet.autheo.com
AUTHEO_EVM_EXPLORER_URL=https://evm-explorer.testnet.autheo.com
AUTHEO_COSMOS_EXPLORER_URL=https://cosmos.testnet.autheo.com
AUTHEO_CHAIN_ID=21270
AUTHEO_CONTRACT_ADDRESS=
AUTHEO_PRIVATE_KEY=
AUTHEO_CONFIRMATIONS=2
AUTHEO_TX_TIMEOUT=2m
AUTHEO_MAX_RETRIES=5
```

`AUTHEO_PRIVATE_KEY` следует получить для отдельного тестового кошелька и хранить только локально в `.env`; не добавлять его в Git и не использовать ключ от Mainnet в разработке. Для Compose ключ передаётся только сервису workflow, а API получает RPC и адрес контракта для read-only проверки владения.

После создания брони workflow записывает proof в контракт. Проверить владение можно запросом `GET /api/v1/bookings/{booking_id}/ownership?wallet_address=0x...`; API вызывает `isBookingOwned` в Autheo и возвращает `owned`, статус proof и transaction hash.

Контракт proof: [`blockchain-deployer/src/BookingProof.sol`](../../blockchain-deployer/src/BookingProof.sol). Он поддерживает создание proof авторизованным backend minter и read-only проверку принадлежности по `bookingHash` и wallet address.
Развернутый контракт Autheo Testnet: `0xb692755BAB064d32efB10F6853039861A3225e43`. Transaction hash: `0xd2651e84888101fb376acfe855075cf08cf6880e0e8850958555faca83ed1d56`. Адрес сохранён в локальном `.env` как `AUTHEO_CONTRACT_ADDRESS`.
