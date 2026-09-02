.PHONY: dev-backend dev-frontend up down migrate seed test build build-mac run-mac

DATABASE_URL ?= postgres://postgres:postgrespassword@localhost:5432/platform_db?sslmode=disable
DEV_TENANT_HOST ?= nexus.localhost
DEV_TENANT_ORIGIN ?= http://$(DEV_TENANT_HOST):3000
DEV_CONTROL_PLANE_HOST ?= admin.localhost

dev-backend:
	cd backend && PUBLIC_ORIGIN="$(DEV_TENANT_ORIGIN)" \
		ALLOWED_ORIGINS="$(DEV_TENANT_ORIGIN),http://$(DEV_CONTROL_PLANE_HOST):3000" \
		CONTROL_PLANE_HOST="$(DEV_CONTROL_PLANE_HOST)" go run ./cmd/api

dev-frontend:
	cd frontend && CONTROL_PLANE_HOST="$(DEV_CONTROL_PLANE_HOST)" \
		NEXT_PUBLIC_API_URL=http://$(DEV_TENANT_HOST):8080/api/v1 \
		NEXT_PUBLIC_CONTROL_PLANE_API_URL=http://$(DEV_CONTROL_PLANE_HOST):8080/api/platform/v1 \
		npm run dev

up:
	docker-compose up -d

down:
	docker-compose down -v

migrate:
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate up

seed:
	cd backend && DATABASE_URL="$(DATABASE_URL)" go run ./cmd/api

test:
	cd backend && go test ./...

build:
	cd backend && go build ./...
	cd frontend && npm run build
	cd native-apps/macos && ./build.sh

build-mac:
	cd native-apps/macos && ./build.sh

run-mac: build-mac
	native-apps/macos/GeregeNexusNativeMac
