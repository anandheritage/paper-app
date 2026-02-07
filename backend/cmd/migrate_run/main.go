package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		fmt.Println("DB_URL not set")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	sql, err := os.ReadFile("migrations/004_enrich_metadata.sql")
	if err != nil {
		fmt.Printf("Failed to read migration: %v\n", err)
		os.Exit(1)
	}

	_, err = pool.Exec(ctx, string(sql))
	if err != nil {
		fmt.Printf("Migration failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Migration 004 applied successfully!")
}
