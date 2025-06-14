# Multi-Tenant Ory Kratos

Ory Kratos modified to load different tenant configs based on `X-Tenant-Id` header.

> **Built with AI** - Started Oct 19, 2024, completed with Claude 4.0 June 13, 2025

## Changes Made

**New Files:**
- `x/tenant_middleware.go` - Extracts tenant ID from headers
- `driver/config/tenant_manager.go` - Caches tenant configs
- `configs/default/kratos.yaml` - Default config
- `configs/tenant1/kratos.yaml` - Example tenant config

**Modified Files:**
- `driver/registry_default.go` - Added tenant fields
- `cmd/serve.go` - Added tenant middleware
- `driver/registry.go` - Added WithTenantAware() option

## How to Use

1. **Setup:**
```powershell
mkdir configs\default, configs\tenant1
# Put your kratos.yaml in configs\default\
# Copy and modify for configs\tenant1\
```

2. **Run:**
```powershell
go build -o kratos-mt.exe .
.\kratos-mt.exe serve --config-dir=.\configs
```

3. **Test:**
```powershell
# Default tenant
curl http://127.0.0.1:4433/health/ready

# Tenant1
curl -H "X-Tenant-Id: tenant1" http://127.0.0.1:4433/health/ready
```

## How It Works

Request → Middleware extracts tenant ID → Loads `/configs/{tenantId}/kratos.yaml` → Falls back to default if missing

**Security:** Sanitizes tenant ID to prevent path traversal
**Performance:** Caches configs in memory with thread-safe access
**Compatibility:** Existing setups work as "default" tenant

## What's Actually Working

✅ **HTTP header extraction** - Gets tenant ID from `X-Tenant-Id`
✅ **Config loading** - Loads from `/configs/{tenantId}/kratos.yaml`
✅ **Fallback** - Uses default config if tenant config missing
✅ **Caching** - Thread-safe config caching with RWMutex
✅ **Security** - Path traversal protection and input sanitization
✅ **File watching** - Hot-reload using configx.AttachWatcher

## Configuration Example

**Default:** `configs/default/kratos.yaml`
```yaml
version: v0.13.0
dsn: memory
serve:
  public:
    base_url: http://127.0.0.1:4433/
selfservice:
  default_browser_return_url: http://127.0.0.1:4455/
identity:
  default_schema_id: default
```

**Tenant:** `configs/tenant1/kratos.yaml`
```yaml
version: v0.13.0
dsn: postgres://tenant1-db:5432/kratos
serve:
  public:
    base_url: http://tenant1.localhost:4433/
selfservice:
  default_browser_return_url: http://tenant1.localhost:4455/
identity:
  default_schema_id: tenant1
```

## Testing

```powershell
# Run unit tests
go test ./driver -v -run "TestTenant"

# Test security (should fallback to default)
curl -H "X-Tenant-Id: ../../../etc/passwd" http://127.0.0.1:4433/health/ready

# Test missing tenant (should fallback to default)
curl -H "X-Tenant-Id: nonexistent" http://127.0.0.1:4433/health/ready
```

## Migration from Single-Tenant

Your existing setup works as "default" tenant with zero changes:

```powershell
# Backup and move existing config
Copy-Item kratos.yaml kratos.yaml.backup
New-Item -Path "configs\default" -ItemType Directory -Force
Move-Item kratos.yaml configs\default\kratos.yaml

# Change startup command
.\kratos serve --config-dir=.\configs  # instead of --config kratos.yaml
```

## Known Issues

- SQLite build requires `CGO_ENABLED=1 go build -tags sqlite`
- File watchers may need filesystem event support
- Large number of tenants will use more memory

---

*Started Oct 19, 2024 - Completed with Claude 4.0 June 13, 2025*
