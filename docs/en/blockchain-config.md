# Autheo configuration

The Autheo integration will use Testnet during development and switch to Mainnet through configuration, without code changes. Environment values live in the root `.env`, which is ignored by Git. The variable template is [`.env.example`](../../.env.example).

Current Testnet settings: EVM RPC `https://rpc.testnet.autheo.com`, Cosmos RPC `https://cosmos-rpc.testnet.autheo.com`, REST API `https://rest.testnet.autheo.com`, chain ID `21270`, currency `THEO` (18 decimals). Faucet: `https://faucet.testnet.autheo.com`. Explorers: `https://evm-explorer.testnet.autheo.com` and `https://cosmos.testnet.autheo.com`. The old Testnet with chain ID `785` is being retired and should not be used. Before switching to Mainnet, update `AUTHEO_NETWORK`, `AUTHEO_RPC_URL`, `AUTHEO_CHAIN_ID`, and the contract address together.

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

Use a separate test wallet and keep `AUTHEO_PRIVATE_KEY` only in the local `.env`; never commit it or use a Mainnet key for development. Compose passes the key only to the workflow service, while the API receives the RPC URL and contract address for read-only ownership checks.

After a booking is created, workflow writes the proof to the contract. Ownership can be checked with `GET /api/v1/bookings/{booking_id}/ownership?wallet_address=0x...`; the API calls `isBookingOwned` on Autheo and returns `owned`, proof status, and the transaction hash.

Proof contract: [`blockchain-deployer/src/BookingProof.sol`](../../blockchain-deployer/src/BookingProof.sol). It supports proof creation by an authorized backend minter and read-only ownership verification by `bookingHash` and wallet address.
Deployed Autheo Testnet contract: `0xb692755BAB064d32efB10F6853039861A3225e43`. Transaction hash: `0xd2651e84888101fb376acfe855075cf08cf6880e0e8850958555faca83ed1d56`. The address is stored in the local `.env` as `AUTHEO_CONTRACT_ADDRESS`.
