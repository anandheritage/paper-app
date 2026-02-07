package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DB_URL")
	}
	if dbURL == "" {
		fmt.Println("DATABASE_URL not set")
		os.Exit(1)
	}
	// Ensure sslmode is set for Railway proxy
	if !strings.Contains(dbURL, "sslmode=") {
		sep := "?"
		if strings.Contains(dbURL, "?") {
			sep = "&"
		}
		dbURL += sep + "sslmode=require"
	}
	fmt.Printf("Connecting to: %s\n", dbURL[:40]+"...")

	ctx := context.Background()
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		fmt.Printf("Failed to parse config: %v\n", err)
		os.Exit(1)
	}
	config.MaxConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		fmt.Printf("Ping failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Connected to database")

	batchSize := 10000
	totalUpdated := int64(0)

	fmt.Println("Backfilling primary_category and categories from metadata in batches...")

	for {
		batchCtx, cancel := context.WithTimeout(ctx, 60*time.Second)

		res, err := pool.Exec(batchCtx, `
			UPDATE papers
			SET
				primary_category = metadata->'categories'->>0,
				categories = (
					SELECT ARRAY(
						SELECT jsonb_array_elements_text(metadata->'categories')
					)
				)
			WHERE id IN (
				SELECT id FROM papers
				WHERE (primary_category IS NULL OR primary_category = '')
				  AND metadata IS NOT NULL
				  AND metadata->'categories' IS NOT NULL
				  AND jsonb_array_length(metadata->'categories') > 0
				LIMIT $1
			)
		`, batchSize)
		cancel()

		if err != nil {
			fmt.Printf("Batch failed: %v (retrying in 3s...)\n", err)
			time.Sleep(3 * time.Second)
			// Retry once
			batchCtx2, cancel2 := context.WithTimeout(ctx, 60*time.Second)
			res, err = pool.Exec(batchCtx2, `
				UPDATE papers
				SET
					primary_category = metadata->'categories'->>0,
					categories = (
						SELECT ARRAY(
							SELECT jsonb_array_elements_text(metadata->'categories')
						)
					)
				WHERE id IN (
					SELECT id FROM papers
					WHERE (primary_category IS NULL OR primary_category = '')
					  AND metadata IS NOT NULL
					  AND metadata->'categories' IS NOT NULL
					  AND jsonb_array_length(metadata->'categories') > 0
					LIMIT $1
				)
			`, batchSize)
			cancel2()
			if err != nil {
				fmt.Printf("Retry also failed: %v\n", err)
				fmt.Printf("Total updated so far: %d\n", totalUpdated)
				os.Exit(1)
			}
		}

		affected := res.RowsAffected()
		totalUpdated += affected
		fmt.Printf("  Batch: %d rows (total: %d)\n", affected, totalUpdated)

		if affected == 0 {
			break
		}

		// Brief pause to not overwhelm the DB
		time.Sleep(1 * time.Second)
	}

	fmt.Printf("\nDone! Total updated: %d rows\n", totalUpdated)
}
