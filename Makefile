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

# ──────────────────────────────────────────────
# Local Docker (development)
# ──────────────────────────────────────────────

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-build:
	docker-compose build

docker-logs:
	docker-compose logs -f

# ──────────────────────────────────────────────
# AWS EC2 Production
# ──────────────────────────────────────────────

# Deploy to EC2 (run from EC2 instance, or use ssh)
prod-deploy:
	./deploy/deploy.sh deploy

prod-restart:
	./deploy/deploy.sh restart

prod-rebuild:
	./deploy/deploy.sh rebuild

prod-logs:
	./deploy/deploy.sh logs

prod-status:
	./deploy/deploy.sh status

prod-stop:
	./deploy/deploy.sh stop

prod-backup:
	./deploy/deploy.sh backup

# ──────────────────────────────────────────────
# Database migrations
# ──────────────────────────────────────────────

migrate-up:
	cd backend && go run cmd/migrate/main.go up

migrate-down:
	cd backend && go run cmd/migrate/main.go down

# ──────────────────────────────────────────────
# arXiv Metadata Ingestion & Enrichment
# ──────────────────────────────────────────────

# Download the Kaggle arXiv metadata snapshot (~1.4 GB zip → ~4 GB JSON)
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
# OAI-PMH Harvesting & OpenSearch Indexing
# ──────────────────────────────────────────────

# Harvest ALL arXiv metadata via OAI-PMH
harvest:
	@echo "Harvesting from arXiv OAI-PMH..."
	cd backend && go run cmd/harvest/main.go

harvest-cs:
	cd backend && go run cmd/harvest/main.go --set=cs

harvest-math:
	cd backend && go run cmd/harvest/main.go --set=math

harvest-physics:
	cd backend && go run cmd/harvest/main.go --set=physics

harvest-resume:
	cd backend && go run cmd/harvest/main.go --resume

harvest-test:
	cd backend && go run cmd/harvest/main.go --max=1000

# Index papers from PostgreSQL into OpenSearch
index:
	@echo "Indexing papers into OpenSearch..."
	cd backend && go run cmd/index/main.go

index-recreate:
	cd backend && go run cmd/index/main.go --recreate

# Full pipeline: harvest → enrich → index
pipeline: harvest enrich index

# ──────────────────────────────────────────────
# Deployment (Vercel frontend)
# ──────────────────────────────────────────────

deploy-frontend:
	cd frontend && npx vercel --prod --yes

# ──────────────────────────────────────────────
# Utilities
# ──────────────────────────────────────────────

clean:
	rm -rf frontend/dist
	rm -rf backend/bin
	docker-compose down -v

install: frontend-install
	cd backend && go mod download

test: frontend-test backend-test

lint:
	cd frontend && npm run lint
	cd backend && golangci-lint run
