package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
	"log/slog"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type envelope struct {
	EventID   string `json:"event_id"`
	BookingID string `json:"aggregate_id"`
	Type      string `json:"event_type"`
	Payload   struct {
		WalletAddress string `json:"wallet_address"`
	} `json:"payload"`
}

const contractABI = `[{"inputs":[{"internalType":"bytes32","name":"bookingHash","type":"bytes32"},{"internalType":"address","name":"owner","type":"address"}],"name":"createBookingProof","outputs":[{"internalType":"uint256","name":"proofId","type":"uint256"}],"stateMutability":"nonpayable","type":"function"}]`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	l := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	db, err := pgxpool.New(ctx, getenv("DATABASE_URL", "postgres://debook:debook_local@postgres:5432/debook"))
	if err != nil {
		l.Error("db", "error", err)
		return
	}
	defer db.Close()
	r := kafka.NewReader(kafka.ReaderConfig{Brokers: []string{getenv("KAFKA_BROKERS", "kafka:19092")}, Topic: getenv("KAFKA_TOPIC", "booking.events"), GroupID: "booking-workflow", MinBytes: 1, MaxBytes: 10e6})
	defer r.Close()
	rpcURL := getenv("AUTHEO_RPC_URL", "")
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		l.Error("autheo rpc", "error", err)
		return
	}
	defer client.Close()
	key := strings.TrimPrefix(getenv("AUTHEO_PRIVATE_KEY", ""), "0x")
	privateKey, err := crypto.HexToECDSA(key)
	if err != nil {
		l.Error("autheo private key", "error", err)
		return
	}
	chainID, _ := new(big.Int).SetString(getenv("AUTHEO_CHAIN_ID", "21270"), 10)
	parsed, _ := abi.JSON(strings.NewReader(contractABI))
	contract := bind.NewBoundContract(common.HexToAddress(getenv("AUTHEO_CONTRACT_ADDRESS", "")), parsed, client, client, client)
	for {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			return
		}
		var e envelope
		if json.Unmarshal(m.Value, &e) != nil || e.Type != "BookingReserved" {
			continue
		}
		if err := run(ctx, db, client, contract, privateKey, chainID, e); err != nil {
			l.Error("blockchain proof", "booking_id", e.BookingID, "error", err)
			continue
		}
		l.Info("booking event processed", "booking_id", e.BookingID)
	}
}
func run(ctx context.Context, db *pgxpool.Pool, client *ethclient.Client, contract *bind.BoundContract, key *ecdsa.PrivateKey, chainID *big.Int, e envelope) error {
	var wallet string
	if err := db.QueryRow(ctx, `SELECT COALESCE(wallet_address,'') FROM bookings WHERE id=$1`, e.BookingID).Scan(&wallet); err != nil {
		return err
	}
	if !common.IsHexAddress(wallet) {
		_, _ = db.Exec(ctx, `UPDATE bookings SET status='FAILED',updated_at=now() WHERE id=$1 AND status='PENDING_CONFIRMATION'`, e.BookingID)
		_, _ = db.Exec(ctx, `INSERT INTO booking_steps(booking_id,step,status,attempt,last_error) VALUES($1,'blockchain','FAILED',1,$2) ON CONFLICT (booking_id,step) DO UPDATE SET status='FAILED',last_error=$2,updated_at=now()`, e.BookingID, "invalid wallet_address")
		return nil
	}
	var status, txHash string
	err := db.QueryRow(ctx, `SELECT status,COALESCE(tx_hash,'') FROM blockchain_proofs WHERE booking_id=$1`, e.BookingID).Scan(&status, &txHash)
	if err == nil && status == "CONFIRMED" {
		return nil
	}
	claim := sha256.Sum256([]byte(e.BookingID))
	hash := "0x" + hex.EncodeToString(claim[:])
	_, err = db.Exec(ctx, `INSERT INTO blockchain_proofs(booking_id,chain_id,contract_address,claim_hash,status) VALUES($1,$2,$3,$4,'PENDING') ON CONFLICT (booking_id) DO NOTHING`, e.BookingID, getenv("AUTHEO_CHAIN_ID", "21270"), getenv("AUTHEO_CONTRACT_ADDRESS", ""), hash)
	if err != nil {
		return err
	}
	bookingHash := [32]byte(claim)
	var txHashValue common.Hash
	if status == "SUBMITTED" && txHash != "" {
		txHashValue = common.HexToHash(txHash)
	} else {
		auth, err := bind.NewKeyedTransactorWithChainID(key, chainID)
		if err != nil {
			return err
		}
		tx, err := contract.Transact(auth, "createBookingProof", bookingHash, common.HexToAddress(wallet))
		if err != nil {
			return err
		}
		txHashValue = tx.Hash()
		if _, err = db.Exec(ctx, `UPDATE blockchain_proofs SET status='SUBMITTED',tx_hash=$2 WHERE booking_id=$1`, e.BookingID, txHashValue.Hex()); err != nil {
			return err
		}
	}
	tx, _, err := client.TransactionByHash(ctx, txHashValue)
	if err != nil {
		return err
	}
	_, err = bind.WaitMined(ctx, client, tx)
	if err != nil {
		return err
	}
	receipt, err := client.TransactionReceipt(ctx, tx.Hash())
	if err != nil {
		return err
	}
	if receipt.Status != 1 {
		return errors.New("blockchain transaction failed")
	}
	_, err = db.Exec(ctx, `UPDATE blockchain_proofs SET status='CONFIRMED',tx_hash=$2,confirmations=1,confirmed_at=now() WHERE booking_id=$1`, e.BookingID, txHashValue.Hex())
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `INSERT INTO booking_steps(booking_id,step,status,attempt) VALUES($1,'blockchain','SUCCEEDED',1) ON CONFLICT (booking_id,step) DO UPDATE SET status='SUCCEEDED',updated_at=now()`, e.BookingID)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `UPDATE bookings SET status='CONFIRMED',updated_at=now() WHERE id=$1 AND status='PENDING_CONFIRMATION'`, e.BookingID)
	return err
}
func getenv(k, f string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return f
}

var _ = time.Second
