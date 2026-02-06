.PHONY: all build run test clean dev frontend backend docker-up docker-down migrate ingest enrich

# Default target
all: build

# Build everything
build: frontend-build backend-build

# Development mode
dev:
	@echo "Starting development servers..."
	@make -j2 frontend-dev backend-dev

# Frontend commands
frontend-dev:
	cd frontend && npm run dev

frontend-build:
	cd frontend && npm run build

frontend-test:
	cd frontend && npm test

frontend-install:
	cd frontend && npm install

# Backend commands
backend-dev:
	cd backend && go run cmd/server/main.go

backend-build:
	cd backend && go build -o bin/server cmd/server/main.go

backend-test:
	cd backend && go test ./...

# Docker commands
docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-build:
	docker-compose build

docker-logs:
	docker-compose logs -f

# Database migrations
migrate-up:
	cd backend && go run cmd/migrate/main.go up

migrate-down:
	cd backend && go run cmd/migrate/main.go down

# ──────────────────────────────────────────────
# arXiv Metadata Ingestion & Enrichment
# ──────────────────────────────────────────────

# Download the Kaggle arXiv metadata snapshot (~1.4 GB zip → ~4 GB JSON)
# Requires: pip install kaggle   and   ~/.kaggle/kaggle.json
download-arxiv:
	@echo "Downloading arXiv metadata from Kaggle..."
	@mkdir -p data
	kaggle datasets download -d Cornell-University/arxiv -p data
	@echo "Extracting..."
	cd data && unzip -o arxiv.zip
	@echo "Done! File: data/arxiv-metadata-oai-snapshot.json"

# Bulk-load arXiv papers into PostgreSQL (all categories)
ingest:
	@echo "Ingesting arXiv metadata into PostgreSQL..."
	cd backend && go run cmd/ingest/main.go \
		--file ../data/arxiv-metadata-oai-snapshot.json \
		--batch 1000

# Ingest only computer-science papers (cs.*) — much smaller, ~800k papers
ingest-cs:
	@echo "Ingesting CS papers into PostgreSQL..."
	cd backend && go run cmd/ingest/main.go \
		--file ../data/arxiv-metadata-oai-snapshot.json \
		--batch 1000 \
		--categories "cs."

# Enrich papers with citation counts from OpenAlex API
# This runs in the background and can be interrupted/resumed
enrich:
	@echo "Enriching papers with OpenAlex citation counts..."
	cd backend && go run cmd/enrich/main.go

# Quick test: ingest first 10k papers (for development)
ingest-test:
	cd backend && go run cmd/ingest/main.go \
		--file ../data/arxiv-metadata-oai-snapshot.json \
		--batch 500 \
		--limit 10000

# Full pipeline: download → ingest → enrich
data-pipeline: download-arxiv ingest enrich

# ──────────────────────────────────────────────

# Deployment
deploy-backend:
	cd backend && railway up --no-gitignore --detach

deploy-frontend:
	cd frontend && npx vercel --prod --yes

deploy: deploy-backend deploy-frontend

# Clean build artifacts
clean:
	rm -rf frontend/dist
	rm -rf backend/bin
	docker-compose down -v

# Install all dependencies
install: frontend-install
	cd backend && go mod download

# Run all tests
test: frontend-test backend-test

# Lint
lint:
	cd frontend && npm run lint
	cd backend && golangci-lint run
