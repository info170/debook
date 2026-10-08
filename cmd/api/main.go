package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/debook/service/internal/booking"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	db, err := pgxpool.New(ctx, getenv("DATABASE_URL", "postgres://debook:debook_local@localhost:5432/debook"))
	if err != nil {
		logger.Error("db config", "error", err)
		return
	}
	defer db.Close()
	if err = db.Ping(ctx); err != nil {
		logger.Error("db unavailable", "error", err)
		return
	}
	cache := redis.NewClient(&redis.Options{Addr: getenv("REDIS_ADDR", "redis:6379")})
	svc := &booking.Service{DB: db, Cache: cache}
	metrics := &apiMetrics{}
	var chainClient *ethclient.Client
	var chainContract *bind.BoundContract
	if rpc := getenv("AUTHEO_RPC_URL", ""); rpc != "" && getenv("AUTHEO_CONTRACT_ADDRESS", "") != "" {
		if c, e := ethclient.Dial(rpc); e == nil {
			if parsed, e := abi.JSON(strings.NewReader(`[ {"inputs":[{"internalType":"bytes32","name":"bookingHash","type":"bytes32"},{"internalType":"address","name":"owner","type":"address"}],"name":"isBookingOwned","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"} ]`)); e == nil {
				chainClient = c
				chainContract = bind.NewBoundContract(common.HexToAddress(getenv("AUTHEO_CONTRACT_ADDRESS", "")), parsed, c, c, c)
			}
		}
	}
	if chainClient != nil {
		defer chainClient.Close()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "debook_http_requests_total %d\n", atomic.LoadUint64(&metrics.requests))
		fmt.Fprintf(w, "debook_http_errors_total %d\n", atomic.LoadUint64(&metrics.errors))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		if err := db.Ping(context.Background()); err != nil {
			booking.HTTPError(w, 503, err)
			return
		}
		if err := cache.Ping(context.Background()).Err(); err != nil {
			booking.HTTPError(w, 503, err)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /api/v1/slots", func(w http.ResponseWriter, r *http.Request) {
		region := r.URL.Query().Get("region_id")
		resource := r.URL.Query().Get("resource_id")
		statusValue := r.URL.Query().Get("status")
		if region == "" || statusValue == "" {
			booking.HTTPError(w, 400, errors.New("region_id and status are required"))
			return
		}
		statuses := strings.Split(statusValue, ",")
		from, _ := time.Parse(time.RFC3339, r.URL.Query().Get("from"))
		to, _ := time.Parse(time.RFC3339, r.URL.Query().Get("to"))
		if from.IsZero() {
			from = time.Now()
		}
		if to.IsZero() {
			to = from.Add(24 * time.Hour)
		}
		slots, err := svc.Search(r.Context(), region, resource, statuses, from, to)
		if err != nil {
			booking.HTTPError(w, 500, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(slots)
	})
	mux.HandleFunc("POST /api/v1/bookings", func(w http.ResponseWriter, r *http.Request) {
		var req booking.CreateRequest
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			booking.HTTPError(w, 400, errors.New("invalid JSON"))
			return
		}
		b, status, err := svc.Create(r.Context(), r.Header.Get("Idempotency-Key"), req)
		if err != nil {
			booking.HTTPError(w, status, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(b)
	})
	mux.HandleFunc("GET /api/v1/bookings/{booking_id}/ownership", func(w http.ResponseWriter, r *http.Request) {
		if chainContract == nil {
			booking.HTTPError(w, http.StatusServiceUnavailable, errors.New("blockchain verification is not configured"))
			return
		}
		id := r.PathValue("booking_id")
		wallet := r.URL.Query().Get("wallet_address")
		if !common.IsHexAddress(wallet) {
			booking.HTTPError(w, http.StatusBadRequest, errors.New("wallet_address must be a valid EVM address"))
			return
		}
		var claim, status, txHash string
		if err := db.QueryRow(r.Context(), `SELECT claim_hash,status,COALESCE(tx_hash,'') FROM blockchain_proofs WHERE booking_id=$1`, id).Scan(&claim, &status, &txHash); err != nil {
			booking.HTTPError(w, http.StatusNotFound, errors.New("blockchain proof not found"))
			return
		}
		var hash [32]byte
		decoded := strings.TrimPrefix(claim, "0x")
		if _, err := hex.Decode(hash[:], []byte(decoded)); err != nil {
			booking.HTTPError(w, http.StatusInternalServerError, errors.New("invalid stored claim hash"))
			return
		}
		var out []interface{}
		if err := chainContract.Call(&bind.CallOpts{Context: r.Context()}, &out, "isBookingOwned", hash, common.HexToAddress(wallet)); err != nil {
			booking.HTTPError(w, http.StatusBadGateway, err)
			return
		}
		owned := len(out) > 0 && out[0].(bool)
		json.NewEncoder(w).Encode(map[string]any{"booking_id": id, "wallet_address": wallet, "owned": owned, "proof_status": status, "tx_hash": txHash})
	})
	mux.HandleFunc("GET /api/v1/bookings/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/api/v1/bookings/"):]
		b, err := svc.Get(r.Context(), id)
		if err != nil {
			booking.HTTPError(w, 404, err)
			return
		}
		json.NewEncoder(w).Encode(b)
	})
	mux.HandleFunc("POST /api/v1/bookings/", func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) < 9 || r.URL.Path[len(r.URL.Path)-7:] != "/cancel" {
			return
		}
		id := r.URL.Path[len("/api/v1/bookings/") : len(r.URL.Path)-7]
		if err := svc.Cancel(r.Context(), id); err != nil {
			booking.HTTPError(w, 409, err)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "CANCELLED"})
	})
	limit := 120
	if value, err := strconv.Atoi(getenv("RATE_LIMIT_PER_MINUTE", "120")); err == nil && value > 0 {
		limit = value
	}
	server := &http.Server{Addr: ":" + getenv("PORT", "8080"), Handler: cors(security(rateLimit(limit, requestLog(logger, metrics, mux)))), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
}

type apiMetrics struct {
	requests uint64
	errors   uint64
}

func requestLog(l *slog.Logger, m *apiMetrics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddUint64(&m.requests, 1)
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
			r.Header.Set("X-Request-ID", id)
		}
		w.Header().Set("X-Request-ID", id)
		t := time.Now()
		rw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		if rw.status >= 500 {
			atomic.AddUint64(&m.errors, 1)
		}
		l.Info("http request", "request_id", id, "method", r.Method, "path", r.URL.Path, "status", rw.status, "duration_ms", time.Since(t).Milliseconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *statusWriter) Write(b []byte) (int, error) { return w.ResponseWriter.Write(b) }

func security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.Method == http.MethodPost {
			r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		}
		next.ServeHTTP(w, r)
	})
}

func rateLimit(perMinute int, next http.Handler) http.Handler {
	type visitor struct {
		at    time.Time
		count int
	}
	var mu sync.Mutex
	visitors := map[string]visitor{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = r.RemoteAddr
		}
		now := time.Now()
		mu.Lock()
		v := visitors[ip]
		if now.Sub(v.at) >= time.Minute {
			v = visitor{at: now}
		}
		v.count++
		visitors[ip] = v
		mu.Unlock()
		if v.count > perMinute {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func getenv(k, f string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return f
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := map[string]bool{}
		for _, value := range strings.Split(getenv("CORS_ALLOWED_ORIGINS", "http://localhost:8081,http://127.0.0.1:8081,http://localhost:5173,http://127.0.0.1:5173"), ",") {
			allowed[strings.TrimSpace(value)] = true
		}
		// Local Vite may move to another port when the default one is busy.
		// Permit loopback origins while keeping non-local origins allow-listed.
		localOrigin := strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "http://127.0.0.1:")
		if allowed[origin] || localOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key, Authorization")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
