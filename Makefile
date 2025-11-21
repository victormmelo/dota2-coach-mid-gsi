.PHONY: help up down dev docker frontend backend stop-dev clean

help: ## Mostra esta mensagem de ajuda
	@echo "Comandos disponíveis:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

docker: ## Inicia os containers do Docker Compose
	@echo "📦 Iniciando Docker Compose..."
	docker compose up -d
	@echo "✅ Docker Compose iniciado"

frontend: ## Inicia o frontend (Next.js)
	@echo "🎨 Iniciando Frontend (Next.js)..."
	cd dota-dashboard && npm run dev

backend: ## Inicia o backend (Go)
	@echo "⚙️  Iniciando Backend (Go)..."
	go run main.go

dev: ## Roda tudo em paralelo (docker + frontend + backend)
	@bash -c '\
	set -e; \
	echo "🚀 Iniciando todos os serviços..."; \
	echo ""; \
	echo "📦 Iniciando Docker Compose..."; \
	docker compose up -d; \
	echo "✅ Docker Compose iniciado"; \
	echo ""; \
	echo "🎨 Iniciando Frontend (Next.js) em background..."; \
	cd dota-dashboard && npm run dev > /tmp/frontend.log 2>&1 & \
	echo $$! > /tmp/frontend.pid; \
	echo "✅ Frontend iniciado (PID: $$(cat /tmp/frontend.pid))"; \
	echo ""; \
	echo "⚙️  Iniciando Backend (Go)..."; \
	echo "💡 Para parar todos os serviços, pressione Ctrl+C"; \
	echo ""; \
	trap "echo \"\"; echo \"🛑 Parando Frontend...\"; kill $$(cat /tmp/frontend.pid) 2>/dev/null || true; rm -f /tmp/frontend.pid /tmp/frontend.log; exit" INT TERM EXIT; \
	go run main.go'

stop-dev: ## Para o frontend que está rodando em background
	@if [ -f /tmp/frontend.pid ]; then \
		echo "🛑 Parando Frontend (PID: $$(cat /tmp/frontend.pid))..."; \
		kill $$(cat /tmp/frontend.pid) 2>/dev/null || true; \
		rm -f /tmp/frontend.pid /tmp/frontend.log; \
		echo "✅ Frontend parado"; \
	else \
		echo "⚠️  Nenhum Frontend rodando em background"; \
	fi

down: ## Para todos os containers do Docker Compose
	@echo "🛑 Parando Docker Compose..."
	docker compose down
	@echo "✅ Docker Compose parado"

clean: down ## Para containers e remove volumes
	@echo "🧹 Removendo volumes..."
	docker compose down -v
	@echo "✅ Volumes removidos"
