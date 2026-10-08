package main

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type event struct {
	ID, AggregateID, Type      string
	SchemaVersion              int
	Payload                    json.RawMessage
	CorrelationID, CausationID *string
	CreatedAt                  time.Time
}

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
	w := &kafka.Writer{Addr: kafka.TCP(getenv("KAFKA_BROKERS", "kafka:19092")), Topic: getenv("KAFKA_TOPIC", "booking.events"), Balancer: &kafka.Hash{}, BatchTimeout: 100 * time.Millisecond}
	defer w.Close()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := publish(ctx, db, w, l); err != nil {
				l.Error("outbox relay", "error", err)
			}
		}
	}
}
func publish(ctx context.Context, db *pgxpool.Pool, w *kafka.Writer, l *slog.Logger) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id,aggregate_id,type,schema_version,payload,correlation_id,causation_id,created_at FROM outbox_events WHERE published_at IS NULL ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 100`)
	if err != nil {
		return err
	}
	var events []event
	for rows.Next() {
		var e event
		if err = rows.Scan(&e.ID, &e.AggregateID, &e.Type, &e.SchemaVersion, &e.Payload, &e.CorrelationID, &e.CausationID, &e.CreatedAt); err != nil {
			rows.Close()
			return err
		}
		events = append(events, e)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, e := range events {
		body, _ := json.Marshal(map[string]any{"event_id": e.ID, "event_type": e.Type, "schema_version": e.SchemaVersion, "aggregate_id": e.AggregateID, "occurred_at": e.CreatedAt, "correlation_id": e.CorrelationID, "causation_id": e.CausationID, "producer": "outbox-relay", "payload": json.RawMessage(e.Payload)})
		if err = w.WriteMessages(ctx, kafka.Message{Key: []byte(e.AggregateID), Value: body, Headers: []kafka.Header{{Key: "event_id", Value: []byte(e.ID)}}}); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE outbox_events SET published_at=now() WHERE id=$1`, e.ID); err != nil {
			return err
		}
		l.Info("published event", "id", e.ID, "type", e.Type)
	}
	return tx.Commit(ctx)
}
func getenv(k, f string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return f
}
