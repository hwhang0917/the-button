.PHONY: all web bin help

all: web bin ## build frontend then the single binary

help: ## list targets
	@grep -E '^[a-z]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-6s %s\n", $$1, $$2}'

web: ## build the Vite frontend into web/dist
	cd web && npm run build

bin: ## build the Go binary (embeds web/dist)
	go build -o thebutton .
