package db

import (
	"context"
	"log"
	"net"

	"github.com/jackc/pgx/v5/pgxpool"
)

// IPv4OnlyResolver filters DNS results to return only IPv4 addresses
type IPv4OnlyResolver struct{}

func (r *IPv4OnlyResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		return nil, err
	}

	var ipv4addrs []string
	for _, addr := range addrs {
		if ip := net.ParseIP(addr); ip != nil && ip.To4() != nil {
			ipv4addrs = append(ipv4addrs, addr)
		}
	}

	if len(ipv4addrs) == 0 {
		return addrs, nil // fallback to all addresses if no IPv4 found
	}
	return ipv4addrs, nil
}

func NewPool(databaseURL string) *pgxpool.Pool {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		log.Fatalf("failed to parse database URL: %v", err)
	}
	cfg.MaxConns = 10

	// Force IPv4 only for Render deployment (IPv6 not available)
	cfg.ConnConfig.Dialer = &net.Dialer{
		Timeout:  cfg.ConnConfig.ConnectTimeout,
		Resolver: &IPv4OnlyResolver{},
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
