package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	region := flag.String("region", getenv("SEED_REGION", "eu-1"), "slot region")
	resource := flag.String("resource", getenv("SEED_RESOURCE", "room-1"), "slot resource")
	days := flag.Int("days", 365, "number of days to generate")
	startValue := flag.String("start", "", "UTC start time in RFC3339; defaults to next whole UTC minute")
	flag.Parse()
	if *days < 1 {
		log.Fatal("days must be greater than zero")
	}
	start := time.Now().UTC().Truncate(time.Minute).Add(time.Minute)
	if *startValue != "" {
		parsed, err := time.Parse(time.RFC3339, *startValue)
		if err != nil {
			log.Fatalf("invalid start: %v", err)
		}
		start = parsed.UTC().Truncate(time.Minute)
	}
	count := *days * 24 * 60
	ctx := context.Background()
	db, err := pgxpool.New(ctx, getenv("DATABASE_URL", "postgres://debook:debook_local@localhost:5432/debook"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		log.Fatal(err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback(ctx)
	// Slots are referenced by bookings and Saga records. CASCADE deliberately
	// resets the complete local booking dataset before generating fresh slots.
	if _, err := tx.Exec(ctx, `TRUNCATE booking_steps, bookings, idempotency_keys, outbox_events, slots RESTART IDENTITY CASCADE`); err != nil {
		log.Fatal(err)
	}
	rows := make([][]any, 0, 4096)
	for i := 0; i < count; i++ {
		begin := start.Add(time.Duration(i) * time.Minute)
		rows = append(rows, []any{uuid.New(), *region, *resource, begin, begin.Add(time.Minute), "AVAILABLE", int64(1)})
		if len(rows) == cap(rows) || i == count-1 {
			if _, err := tx.CopyFrom(ctx, pgx.Identifier{"slots"}, []string{"id", "region_id", "resource_id", "starts_at", "ends_at", "status", "version"}, pgx.CopyFromRows(rows)); err != nil {
				log.Fatal(err)
			}
			rows = rows[:0]
		}
	}
	if err := tx.Commit(ctx); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("seeded %d one-minute slots from %s to %s for %s/%s\n", count, start.Format(time.RFC3339), start.Add(time.Duration(count)*time.Minute).Format(time.RFC3339), *region, *resource)
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
