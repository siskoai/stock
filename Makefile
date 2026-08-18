# Comptoir — raccourcis de développement.
#
# La compilation complète passe par la ligne de commande Wails, qui construit
# le frontend puis l'embarque dans l'exécutable :
#     go install github.com/wailsapp/wails/v2/cmd/wails@latest

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: help dev build build-windows test check fmt tidy frontend clean

help: ## Affiche cette aide
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

dev: ## Lance l'application avec rechargement à chaud
	wails dev -ldflags "$(LDFLAGS)"

build: ## Compile l'exécutable pour le système courant
	wails build -clean -ldflags "$(LDFLAGS)"

build-windows: ## Compile pour Windows depuis n'importe quel système
	wails build -clean -platform windows/amd64 -nsis -ldflags "$(LDFLAGS)"

test: ## Exécute la suite de tests Go
	go test ./...

check: test ## Tests, analyse statique et typage du frontend
	go vet ./...
	gofmt -l . | grep -v '^frontend/' | (! grep .) || (echo "fichiers non formatés ci-dessus" && exit 1)
	cd frontend && npm run build

fmt: ## Reformate le code Go
	gofmt -w .

tidy: ## Nettoie les dépendances Go
	go mod tidy

frontend: ## Compile seulement l'interface
	cd frontend && npm install && npm run build

clean: ## Supprime les artefacts de compilation
	rm -rf build/bin frontend/dist/assets frontend/dist/index.html
