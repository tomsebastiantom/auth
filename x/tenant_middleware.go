// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package x

import (
	"context"
	"net/http"
	"strings"

	"github.com/urfave/negroni"
)

// TenantContextKey is the context key for storing tenant information
type TenantContextKey string

const (
	// TenantIDKey is the context key for tenant ID
	TenantIDKey TenantContextKey = "tenant_id"
	// TenantIDHeader is the HTTP header for tenant ID
	TenantIDHeader = "X-Tenant-Id"
	// DefaultTenantID is the fallback tenant when no tenant header is provided
	DefaultTenantID = "default"
)

// TenantMiddleware extracts tenant ID from HTTP headers and adds it to request context
func TenantMiddleware() negroni.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tenantID := extractTenantID(r)
		ctx := context.WithValue(r.Context(), TenantIDKey, tenantID)
		next(rw, r.WithContext(ctx))
	}
}

// extractTenantID extracts tenant ID from the X-Tenant-Id header with fallback to default
func extractTenantID(r *http.Request) string {
	tenantID := strings.TrimSpace(r.Header.Get(TenantIDHeader))
	if tenantID == "" {
		return DefaultTenantID
	}

	// Sanitize tenant ID to prevent path traversal attacks
	tenantID = strings.ReplaceAll(tenantID, "..", "")
	tenantID = strings.ReplaceAll(tenantID, "/", "")
	tenantID = strings.ReplaceAll(tenantID, "\\", "")

	if tenantID == "" {
		return DefaultTenantID
	}

	return tenantID
}

// GetTenantID retrieves tenant ID from context
func GetTenantID(ctx context.Context) string {
	if tenantID, ok := ctx.Value(TenantIDKey).(string); ok {
		return tenantID
	}
	return DefaultTenantID
}

// SetTenantID sets tenant ID in context (useful for testing)
func SetTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, TenantIDKey, tenantID)
}
