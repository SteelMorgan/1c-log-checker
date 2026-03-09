.PHONY: help build test lint clean \
        docker-build infra-up infra-down infra-restart infra-logs infra-status \
        mod-tidy

# =============================================================================
# help
# =============================================================================
help:
	@echo "1C Log Parser — Makefile"
	@echo ""
	@echo "  [ Сборка ]"
	@echo "  build             - Собрать все бинарники (parser, mcp, утилиты)"
	@echo "  clean             - Удалить артефакты сборки"
	@echo "  mod-tidy          - go mod tidy"
	@echo ""
	@echo "  [ Качество кода ]"
	@echo "  test              - Запустить тесты"
	@echo "  lint              - Запустить линтеры"
	@echo ""
	@echo "  [ Docker стек ]"
	@echo "  infra-up          - Собрать образы и поднять все контейнеры"
	@echo "  infra-down        - Остановить все контейнеры"
	@echo "  infra-restart     - Перезапустить стек"
	@echo "  infra-logs        - Логи парсера (live)"
	@echo "  infra-status      - Состояние всех контейнеров + sshfs"
	@echo "  docker-build      - Только пересобрать Docker образы"

# =============================================================================
# Сборка
# =============================================================================
build:
	@echo "Building parser..."
	@go build -o bin/parser.exe ./cmd/parser
	@echo "Building MCP server..."
	@go build -o bin/mcp.exe ./cmd/mcp
	@echo "Building extract_mxl utility..."
	@go build -o bin/extract_mxl.exe ./cmd/extract_mxl
	@echo "Building compare utility..."
	@go build -o bin/compare.exe ./cmd/compare
	@echo "Done!"

test:
	@echo "Running tests..."
	@go test ./... -v -cover

lint:
	@echo "Running linters..."
	@go fmt ./...
	@go vet ./...
	@golangci-lint run

clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -rf build/
	@go clean

mod-tidy:
	@echo "Running go mod tidy..."
	@go mod tidy

# =============================================================================
# Docker стек
# =============================================================================
docker-build:
	@echo "Rebuilding Docker images..."
	@cd deploy/docker && docker compose build

infra-up:
	@echo "[infra] Поднимаем стек (парсер + ClickHouse + Grafana + MCP)..."
	@cd deploy/docker && docker compose up -d --build
	@echo ""
	@echo "  Сервисы:"
	@echo "    ClickHouse : http://localhost:8123"
	@echo "    Grafana    : http://localhost:3000"
	@echo "    MCP Server : http://localhost:8080"
	@echo ""
	@echo "  Проверить состояние: make infra-status"

infra-down:
	@echo "[infra] Останавливаем стек..."
	@cd deploy/docker && docker compose down

infra-restart: infra-down infra-up

infra-logs:
	@cd deploy/docker && docker compose logs -f log-parser

infra-status:
	@echo "=== Docker контейнеры ==="
	@cd deploy/docker && docker compose ps
	@echo ""
	@echo "=== sshfs mounts (parser) ==="
	@docker exec 1c-log-parser sh -c "mountpoint -q /mnt/srvinfo && echo '  srvinfo: OK' && ls /mnt/srvinfo | head -5 || echo '  srvinfo: НЕ примонтировано'" 2>/dev/null || echo "  контейнер парсера не запущен"
	@docker exec 1c-log-parser sh -c "mountpoint -q /mnt/techlog && echo '  techlog: OK' && ls /mnt/techlog | head -5 || echo '  techlog: НЕ примонтировано'" 2>/dev/null || echo "  контейнер парсера не запущен"

# Алиасы для совместимости
docker-up: infra-up
docker-down: infra-down
