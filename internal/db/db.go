package db

import (
	"context"
	"log"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates a pgx connection pool with retry logic for Render deployments.
// Forces IPv4-only connections because Render does not support IPv6 outbound.
// Appends sslmode=require if not already present (required for Supabase).
func NewPool(databaseURL string) *pgxpool.Pool {
	// Ensure sslmode=require is present — mandatory for Supabase from external hosts
	if !strings.Contains(databaseURL, "sslmode=") {
		if strings.Contains(databaseURL, "?") {
			databaseURL += "&sslmode=require"
		} else {
			databaseURL += "?sslmode=require"
		}
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		log.Fatalf("failed to parse database URL: %v", err)
	}

	// Force IPv4-only dialer — Render has no IPv6 outbound support.
	// This overrides pgx's default dialer so TCP connections always use tcp4.
	cfg.ConnConfig.Config.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		d := &net.Dialer{Timeout: 10 * time.Second}
		return d.DialContext(ctx, "tcp4", addr)
	}

	cfg.MaxConns = 10
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		log.Fatalf("failed to create connection pool: %v", err)
	}

	// Retry ping up to 5 times with backoff — Render cold-starts can be slow
	const maxRetries = 5
	for i := 1; i <= maxRetries; i++ {
		pingCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err = pool.Ping(pingCtx)
		cancel()
		if err == nil {
			log.Println("database connected successfully")
			return pool
		}
		log.Printf("database ping attempt %d/%d failed: %v", i, maxRetries, err)
		if i < maxRetries {
			time.Sleep(time.Duration(i*2) * time.Second) // 2s, 4s, 6s, 8s backoff
		}
	}

	log.Fatalf("failed to connect to database after %d attempts: %v", maxRetries, err)
	return nil
}
