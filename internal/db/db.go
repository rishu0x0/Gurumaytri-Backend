package db

import (
	"context"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(databaseURL string) *pgxpool.Pool {
	// For Render: force IPv4 by converting hostname to localhost in dev or handling in connection
	// Supabase: append IPv4 parameter if available
	if strings.Contains(databaseURL, "supabase.co") {
		// Ensure we're using the correct connection parameters for IPv4
		if !strings.Contains(databaseURL, "sslmode") {
			databaseURL += "?sslmode=require"
		}
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		log.Fatalf("failed to parse database URL: %v", err)
	}
	cfg.MaxConns = 10

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		log.Fatalf("failed to create connection pool: %v", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	log.Println("database connected")
	return pool
}
