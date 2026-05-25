package db

import (
	"context"
	"log"
	"net"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(databaseURL string) *pgxpool.Pool {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		log.Fatalf("failed to parse database URL: %v", err)
	}
	cfg.MaxConns = 10
	
	// Force IPv4 only for Render deployment (IPv6 not available)
	cfg.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		d := net.Dialer{
			Timeout: cfg.ConnConfig.ConnectTimeout,
		}
		return d.DialContext(ctx, "tcp4", addr)
	}

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
