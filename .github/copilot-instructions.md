# GitHub Copilot Instructions for terraform-provider-autoglue

## Project overview

- This repo implements the **Autoglue Terraform provider**.
- Terraform source address: `registry.terraform.io/GlueOps/autoglue`.
- The provider is written in **Go**, using:
    - `github.com/hashicorp/terraform-plugin-framework`
    - `github.com/hashicorp/terraform-plugin-docs`
    - `github.com/hashicorp/terraform-plugin-framework-validators`
    - Codegen from `tfplugingen-openapi` and `tfplugingen-framework`.

### Layout

- `main.go`: entrypoint; calls `providerserver.Serve` with `provider.New(version)`.
- `internal/provider/`:
    - Generated files: `*_gen.go` (from tfplugingen).
    - Hand-written provider, resources, data sources, client:
        - `provider.go` – provider config & wiring.
        - `client.go` (or similar) – HTTP client for the Autoglue API.
        - `cluster_resource.go`, `cluster_data_source.go`, `ssh_key_*`, `server_*`,
          `taint_*`, `label_*`, `annotation_*`, `node_pool_*`, attachment resources, etc.
- `docs/`: terraform-plugin-docs output (`make docs` / `make readme`).
- `generator_config.yaml` + `provider_spec.json`: config & output for tfplugingen.

## General guidance

- **Never edit generated files** (anything under `internal/provider/**` ending in `_gen.go`).
    - Those are produced by:
        - `make generate` (tfplugingen-openapi + tfplugingen-framework).
- Prefer adding or modifying **hand-written** files in `internal/provider` that do **not** have the `_gen.go` suffix.
- After changing provider schema / resources / data sources:
    - The human workflow is: `make build` → `make test` (if present) → `make docs` → `make readme`.

## Terraform provider conventions

When creating or updating resources / data sources:

- Implement:
    - `Metadata`, `Schema`, `Configure` for both resources and data sources.
    - `Create`, `Read`, `Update`, `Delete`, and `ImportState` (where appropriate) for resources.
    - `Read` for data sources.
- Use framework types:
    - `types.String`, `types.Bool`, `types.Number`, `types.Set`, `types.List`, `types.Map`, etc.
- Use plan modifiers where appropriate:
    - IDs: `stringplanmodifier.UseStateForUnknown()`.
    - Sets that should not drift: `setplanmodifier.UseStateForUnknown()`.
    - Attributes that require replacement when changed: `stringplanmodifier.RequiresReplace()` or
      `setplanmodifier.RequiresReplace()` as appropriate.

## Autoglue-specific rules

- The provider name is `autoglue`. Resource / data source type names should be:
    - Resources: `autoglue_<thing>` e.g. `autoglue_cluster`, `autoglue_ssh_key`, `autoglue_node_pool`.
    - Data sources: `autoglue_<thing>` e.g. `autoglue_ssh_keys`, `autoglue_cluster`.
- **Provider configuration attributes** (defined in `provider.go`):
    - `base_url` (optional; default is Autoglue SaaS URL).
    - `org_id` (required).
    - `api_key`, `org_key`, `org_secret`, `bearer_token` (optional; some are sensitive).
- Use the shared HTTP client (`autoglueClient`) from `provider.Configure` via:
    - `req.ProviderData` in `Configure` methods for resources / data sources.
- The client helper method `doJSON` should be used for API calls:
    - It handles base URL, headers, and JSON encode/decode.
    - Prefer defining small request/response DTO structs alongside resources when needed.

### Node pools

- Node pools in the API:
    - Role is an enum: `"master"` or `"worker"`.
- Node pool resource:
    - Ensure Terraform schema matches current Autoglue API DTOs:
        - `CreateNodePoolRequest` / `UpdateNodePoolRequest`.
        - `NodePoolResponse`.

### Attachments

- Attachment resources (e.g. node pool servers / labels / taints / annotations) should:
    - Treat the attachment set as authoritative:
        - `server_ids`, `label_ids`, etc. are **sets** in Terraform.
    - On update:
        - Compute the diff between old and new sets.
        - Call the appropriate `/node-pools/{id}/...` attach/detach APIs.
    - Use `types.Set` with `types.StringType` for ID lists.

## Coding style

- Use idiomatic Go:
    - Run `go fmt` (or `gofmt`) on all Go files.
    - Return early on error.
    - Use `tflog` for logging (already imported in many files).
- Keep resource and data source structs small and cohesive.
- Keep Terraform schema
