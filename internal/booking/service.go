package booking

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Service struct {
	DB    *pgxpool.Pool
	Cache *redis.Client
}
type CreateRequest struct {
	SlotID        string `json:"slot_id"`
	CustomerRef   string `json:"customer_ref"`
	WalletAddress string `json:"wallet_address"`
	TenantID      string `json:"tenant_id"`
}
type Booking struct {
	ID            string     `json:"id"`
	SlotID        string     `json:"slot_id"`
	Status        string     `json:"status"`
	CustomerRef   string     `json:"customer_ref"`
	WalletAddress string     `json:"wallet_address"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
}

func (s *Service) Create(ctx context.Context, key string, req CreateRequest) (Booking, int, error) {
	if key == "" || req.SlotID == "" || req.CustomerRef == "" || req.WalletAddress == "" {
		return Booking{}, http.StatusBadRequest, errors.New("slot_id, customer_ref, wallet_address and Idempotency-Key are required")
	}
	if !common.IsHexAddress(req.WalletAddress) {
		return Booking{}, http.StatusBadRequest, errors.New("wallet_address must be a valid EVM address")
	}
	tenant := req.TenantID
	if tenant == "" {
		tenant = "default"
	}
	hashBytes, _ := json.Marshal(req)
	h := sha256.Sum256(hashBytes)
	hash := hex.EncodeToString(h[:])
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Booking{}, 500, err
	}
	defer tx.Rollback(ctx)
	var storedHash string
	var storedStatus *int
	var storedPayload []byte
	err = tx.QueryRow(ctx, `SELECT request_hash,response_status,response_payload FROM idempotency_keys WHERE tenant_id=$1 AND key=$2`, tenant, key).Scan(&storedHash, &storedStatus, &storedPayload)
	if err == nil {
		if storedHash != hash {
			return Booking{}, 409, errors.New("idempotency key reused with different request")
		}
		var b Booking
		if json.Unmarshal(storedPayload, &b) != nil {
			return Booking{}, 500, errors.New("invalid stored response")
		}
		status := 200
		if storedStatus != nil {
			status = *storedStatus
		}
		return b, status, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, 500, err
	}
	var status string
	var expires *time.Time
	if err = tx.QueryRow(ctx, `SELECT status, CASE WHEN ends_at > now() THEN ends_at ELSE now() END FROM slots WHERE id=$1 FOR UPDATE`, req.SlotID).Scan(&status, &expires); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Booking{}, 404, errors.New("slot not found")
		}
		return Booking{}, 500, err
	}
	if status != "AVAILABLE" {
		return Booking{}, 409, errors.New("slot is not available")
	}
	id := uuid.New()
	if _, err = tx.Exec(ctx, `UPDATE slots SET status='RESERVED',version=version+1,updated_at=now() WHERE id=$1`, req.SlotID); err != nil {
		return Booking{}, 500, err
	}
	b := Booking{ID: id.String(), SlotID: req.SlotID, Status: "PENDING_CONFIRMATION", CustomerRef: req.CustomerRef, WalletAddress: req.WalletAddress, ExpiresAt: expires}
	payload, _ := json.Marshal(b)
	if _, err = tx.Exec(ctx, `INSERT INTO bookings(id,slot_id,tenant_id,customer_ref,wallet_address,status,idempotency_key,request_hash,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, req.SlotID, tenant, req.CustomerRef, req.WalletAddress, b.Status, key, hash, expires); err != nil {
		return Booking{}, 500, err
	}
	event, _ := json.Marshal(map[string]any{"booking_id": id.String(), "slot_id": req.SlotID, "status": b.Status, "wallet_address": req.WalletAddress})
	if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(id,aggregate_id,type,payload) VALUES($1,$2,'BookingReserved',$3)`, uuid.New(), id, event); err != nil {
		return Booking{}, 500, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO idempotency_keys(tenant_id,key,request_hash,response_status,response_payload,expires_at) VALUES($1,$2,$3,$4,$5,now()+interval '24 hours')`, tenant, key, hash, 202, payload); err != nil {
		return Booking{}, 500, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Booking{}, 500, err
	}
	return b, 202, nil
}
func (s *Service) Get(ctx context.Context, id string) (Booking, error) {
	var b Booking
	err := s.DB.QueryRow(ctx, `SELECT id,slot_id,status,customer_ref,wallet_address,expires_at FROM bookings WHERE id=$1`, id).Scan(&b.ID, &b.SlotID, &b.Status, &b.CustomerRef, &b.WalletAddress, &b.ExpiresAt)
	return b, err
}
func (s *Service) Cancel(ctx context.Context, id string) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var slot string
	if err = tx.QueryRow(ctx, `SELECT slot_id FROM bookings WHERE id=$1 AND status IN ('PENDING_CONFIRMATION','CONFIRMED') FOR UPDATE`, id).Scan(&slot); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE bookings SET status='CANCELLED',updated_at=now() WHERE id=$1`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE slots SET status='AVAILABLE',version=version+1,updated_at=now() WHERE id=$1`, slot); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func HTTPError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

var _ = fmt.Sprintf
var _ = strings.TrimSpace

func (s *Service) Search(ctx context.Context, region, resource string, statuses []string, from, to time.Time) ([]map[string]any, error) {
	args := []any{region, resource, from, to}
	placeholders := make([]string, len(statuses))
	for i, status := range statuses {
		placeholders[i] = fmt.Sprintf("$%d", len(args)+i+1)
		args = append(args, status)
	}
	query := fmt.Sprintf(`SELECT id,region_id,resource_id,starts_at,ends_at,status FROM slots WHERE region_id=$1 AND ($2='' OR resource_id=$2) AND starts_at >= $3 AND ends_at <= $4 AND status IN (%s) ORDER BY starts_at`, strings.Join(placeholders, ","))
	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, reg, res, status string
		var start, end time.Time
		if err := rows.Scan(&id, &reg, &res, &start, &end, &status); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "region_id": reg, "resource_id": res, "starts_at": start, "ends_at": end, "status": status})
	}
	return out, rows.Err()
}
