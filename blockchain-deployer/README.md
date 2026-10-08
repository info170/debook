# Debook blockchain deployer

This is a standalone project for compiling and deploying smart contracts to Autheo blockchain. It contains the Solidity sources, Foundry configuration, deployment scripts, and a Docker Compose wrapper. The main application project does not start Foundry or depend on this project at runtime.

## Network configuration

Copy the example configuration:

```bash
cp .env.example .env
```

The default network is Autheo Testnet:

```text
RPC:     https://rpc.testnet.autheo.com
Chain ID: 21270
```

Get test THEO from the [Autheo Testnet faucet](https://faucet.testnet.autheo.com). Use a dedicated test wallet.

## Save .sol smart-contract to /src folder

## Compile

```bash
docker compose run --rm foundry build
```

## Deploy with a private key

Change src/BookingProof.sol:BookingProof to your smart-contract filename (if differs).
The private key is supplied only when the command runs. Do not put it in `.env`, the compose file, source code, or chat.

```bash
set -a
source .env
set +a

read -s "AUTHEO_PRIVATE_KEY?Testnet deployer private key: "
export AUTHEO_PRIVATE_KEY
printf '\n'

docker compose run --rm \
  -e AUTHEO_PRIVATE_KEY \
  foundry create src/BookingProof.sol:BookingProof \
  --rpc-url "$AUTHEO_RPC_URL" \
  --chain-id "$AUTHEO_CHAIN_ID" \
  --private-key "$AUTHEO_PRIVATE_KEY" \
  --broadcast

unset AUTHEO_PRIVATE_KEY
```

The output includes `Deployed to` and the transaction hash. Save the public contract address in the main application's `.env` as `AUTHEO_CONTRACT_ADDRESS=0x...` after deployment. The deployer is the initial admin and minter; later grant the backend adapter access with `setMinter(adapterAddress, true)`.

Check the contract: https://evm-explorer.testnet.autheo.com/
