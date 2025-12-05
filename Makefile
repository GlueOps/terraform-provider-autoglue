# Simple Makefile for terraform-provider-autoglue

GO    ?= go
BINARY ?= terraform-provider-autoglue

.PHONY: all build install test generate generate-openapi generate-framework docs clean

all: build

## Build / install / test

build:
	$(GO) build -o $(BINARY) .

install:
	$(GO) install .

test:
	$(GO) test ./...

clean:
	rm -f $(BINARY)
	rm -f provider_spec.json

## Code generation

# 1) OpenAPI -> provider spec JSON
generate-openapi:
	tfplugingen-openapi generate \
	  --config ./generator_config.yaml \
	  --output ./provider_spec.json \
	  ../autoglue/docs/openapi.json

# 2) Provider spec JSON -> framework code
generate-framework:
	tfplugingen-framework generate all \
	  --input ./provider_spec.json \
	  --output ./internal/provider

# Convenience target: run both steps
generate: generate-openapi generate-framework

## Docs + README

# Uses terraform-plugin-docs (tfplugindocs). This will:
# - Format example Terraform configs (if you add them under ./examples)
# - Generate docs into ./docs (using index.md.tmpl etc)
# - Generate/refresh README.md from the provider docs template.
docs:
	TFPLUGINDOCS_PROVIDER_HOSTNAME=registry.terraform.io \
	TFPLUGINDOCS_PROVIDER_NAMESPACE=GlueOps \
	$(GO) run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs \
	  generate \
	  --provider-dir . \
	  -provider-name autoglue

readme: docs
	@echo "Building README.md from docs/"
	# Provider overview
	@cat docs/index.md > README.md

	@echo "" >> README.md
	@echo "## Resources" >> README.md

	# Append each resource doc, demoting its top-level heading
	@for f in docs/resources/*.md; do \
	  echo "" >> README.md; \
	  sed '1s/^# /### /' "$$f" >> README.md; \
	done

	@echo "" >> README.md
	@echo "## Data Sources" >> README.md

	# Append each data source doc, demoting its top-level heading
	@for f in docs/data-sources/*.md; do \
	  echo "" >> README.md; \
	  sed '1s/^# /### /' "$$f" >> README.md; \
	done
