SHELL := bash
MAKEFLAGS += --no-print-directory
WEBHOOK_SECRET ?= secret
APM_SERVER_URL ?=
APM_SERVER_BEARER_TOKEN ?=
OTEL_VERSION=v0.106.0

#######################
## Tools
#######################
export PATH := $(CURDIR)/bin:$(PATH)
OCB ?= $(CURDIR)/bin/builder

## @help:install-ngrok:Install ngrok.
.PHONY: install-ngrok
install-ngrok:
ifeq ($(OS),Darwin)
	brew install ngrok/ngrok/ngrok
else
	$(error "Please install ngrok manually")
endif

## @help:install-ocb:Install ocb.
.PHONY: install-ocb
install-ocb:
	GOBIN=$(CURDIR)/bin go install go.opentelemetry.io/collector/cmd/builder@$(OTEL_VERSION)

## MAKE GOALS
.PHONY: build
build: ## Build the binary
	@$(OCB) --skip-strict-versioning --config builder-config.yml

.PHONY: run
run: ## Run the binary
	@WEBHOOK_SECRET=$(WEBHOOK_SECRET) \
	$(cat .env | xargs) \
	./bin/otelcol-custom --config config.yml

.PHONY: ngrok
ngrok: ## Run ngrok
	ngrok http http://localhost:19418
