.PHONY: all build run test clean dev frontend backend docker-up docker-down migrate

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
