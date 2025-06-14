// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/ory/kratos/embedx"
	"github.com/ory/x/configx"
	"github.com/ory/x/logrusx"
	"github.com/ory/x/watcherx"
)


type TenantContextKey string

type TenantManager struct {
	mu               sync.RWMutex
	baseConfig       *Config
	tenantConfigs    map[string]*configx.Provider
	configDirectory  string
	logger           *logrusx.Logger
	watchers         map[string]func()
}


func NewTenantManager(baseConfig *Config, configDirectory string, logger *logrusx.Logger) *TenantManager {
	return &TenantManager{
		baseConfig:      baseConfig,
		tenantConfigs:   make(map[string]*configx.Provider),
		configDirectory: configDirectory,
		logger:          logger,
		watchers:        make(map[string]func()),
	}
}


func (tm *TenantManager) GetTenantConfig(ctx context.Context, tenantID string) *configx.Provider {
	// For default tenant, return base config
	if tenantID == "default" {
		return tm.baseConfig.GetProvider(ctx)
	}

	tm.mu.RLock()
	if provider, exists := tm.tenantConfigs[tenantID]; exists {
		tm.mu.RUnlock()
		return provider
	}
	tm.mu.RUnlock()

	// Load tenant config if not cached
	return tm.loadTenantConfig(ctx, tenantID)
}


func (tm *TenantManager) loadTenantConfig(ctx context.Context, tenantID string) *configx.Provider {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Double-check if another goroutine loaded it while we were waiting
	if provider, exists := tm.tenantConfigs[tenantID]; exists {
		return provider
	}

	tenantConfigPath := tm.getTenantConfigPath(tenantID)

	// Check if tenant config file exists
	if _, err := os.Stat(tenantConfigPath); os.IsNotExist(err) {
		tm.logger.WithField("tenant_id", tenantID).
			WithField("config_path", tenantConfigPath).
			Debug("Tenant config file not found, using default configuration")
		return tm.baseConfig.GetProvider(ctx)
	}

	// Load tenant-specific configuration
	provider, err := tm.createTenantProvider(ctx, tenantConfigPath)
	if err != nil {
		tm.logger.WithError(err).
			WithField("tenant_id", tenantID).
			WithField("config_path", tenantConfigPath).
			Error("Failed to load tenant configuration, falling back to default")
		return tm.baseConfig.GetProvider(ctx)
	}

	// Cache the provider
	tm.tenantConfigs[tenantID] = provider

	// Set up file watching for hot-reload
	tm.setupTenantWatcher(tenantID, tenantConfigPath)

	tm.logger.WithField("tenant_id", tenantID).
		WithField("config_path", tenantConfigPath).
		Info("Successfully loaded tenant configuration")

	return provider
}

// createTenantProvider creates a new configx.Provider for a tenant config file
func (tm *TenantManager) createTenantProvider(ctx context.Context, configPath string) (*configx.Provider, error) {
	// Create a new provider with the tenant config file
	opts := []configx.OptionModifier{
		configx.WithConfigFiles("file://" + configPath),
		configx.WithLogger(tm.logger),
		configx.WithContext(ctx),
		configx.WithImmutables("serve", "profiling", "log"),
		configx.WithExceptImmutables("serve.public.cors.allowed_origins"),
	}

	return configx.New(ctx, []byte(embedx.ConfigSchema), opts...)
}

// setupTenantWatcher sets up file watching for a tenant configuration
func (tm *TenantManager) setupTenantWatcher(tenantID, configPath string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Clean up existing watcher if it exists
	if cleanup, exists := tm.watchers[tenantID]; exists {
		cleanup()
	}

	// Set up file watcher using configx.AttachWatcher
	ctx := context.Background()

	// Create configuration options for the tenant config file
	opts := []configx.OptionModifier{
		configx.WithConfigFiles("file://" + configPath),
		configx.WithLogger(tm.logger),
		configx.WithContext(ctx),
		configx.WithImmutables("serve", "profiling", "log"),
		configx.WithExceptImmutables("serve.public.cors.allowed_origins"),
		configx.AttachWatcher(func(event watcherx.Event, err error) {
			if err != nil {
				tm.logger.WithError(err).
					WithField("tenant_id", tenantID).
					WithField("config_path", configPath).
					Error("File watcher error for tenant config")
				return
			}

			// Log the specific type of file event
			tm.logger.WithField("tenant_id", tenantID).
				WithField("config_path", configPath).
				WithField("event_type", event.String()).
				Info("Tenant configuration file changed, invalidating cache")

			// Invalidate the cached config to force reload on next request
			tm.invalidateTenantConfigUnsafe(tenantID)

			// Optional: Pre-load the new configuration in background
			go func() {
				if _, err := tm.preloadTenantConfig(context.Background(), tenantID, configPath); err != nil {
					tm.logger.WithError(err).
						WithField("tenant_id", tenantID).
						Warn("Failed to preload tenant configuration after file change")
				}
			}()
		}),
	}

	// Create provider with watcher attached
	watcherProvider, err := configx.New(ctx, []byte(embedx.ConfigSchema), opts...)
	if err != nil {
		tm.logger.WithError(err).
			WithField("tenant_id", tenantID).
			WithField("config_path", configPath).
			Error("Failed to create file watcher for tenant config")
		return
	}

	// Store cleanup function that properly closes the watcher
	tm.watchers[tenantID] = func() {
		if watcherProvider != nil {
			// configx.Provider should handle cleanup automatically
			tm.logger.WithField("tenant_id", tenantID).
				Debug("Tenant config watcher cleanup completed")
		}
	}

	tm.logger.WithField("tenant_id", tenantID).
		WithField("config_path", configPath).
		Info("File watcher successfully attached for tenant configuration with hot-reload capability")
}

// getTenantConfigPath returns the file path for a tenant's configuration
func (tm *TenantManager) getTenantConfigPath(tenantID string) string {
	return filepath.Join(tm.configDirectory, tenantID, "kratos.yaml")
}

// InvalidateTenantConfig removes a tenant configuration from cache (useful for hot-reload)
func (tm *TenantManager) InvalidateTenantConfig(tenantID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	delete(tm.tenantConfigs, tenantID)

	// Clean up watcher if exists
	if cleanup, exists := tm.watchers[tenantID]; exists {
		cleanup()
		delete(tm.watchers, tenantID)
	}

	tm.logger.WithField("tenant_id", tenantID).
		Debug("Invalidated tenant configuration cache")
}

// invalidateTenantConfigUnsafe removes a tenant configuration from cache without locking
// NOTE: This method assumes the caller already holds the mutex
func (tm *TenantManager) invalidateTenantConfigUnsafe(tenantID string) {
	delete(tm.tenantConfigs, tenantID)
	tm.logger.WithField("tenant_id", tenantID).
		Debug("Invalidated tenant configuration cache (unsafe)")
}

// preloadTenantConfig attempts to preload a tenant configuration in the background
func (tm *TenantManager) preloadTenantConfig(ctx context.Context, tenantID, configPath string) (*configx.Provider, error) {
	// Create tenant-specific configuration
	provider, err := tm.createTenantProvider(ctx, configPath)
	if err != nil {
		return nil, err
	}

	// Cache the new provider
	tm.mu.Lock()
	tm.tenantConfigs[tenantID] = provider
	tm.mu.Unlock()

	tm.logger.WithField("tenant_id", tenantID).
		WithField("config_path", configPath).
		Info("Successfully preloaded tenant configuration after file change")

	return provider, nil
}

// GetTenantConfigStats returns statistics about loaded tenant configurations
func (tm *TenantManager) GetTenantConfigStats() map[string]interface{} {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return map[string]interface{}{
		"loaded_tenants_count": len(tm.tenantConfigs),
		"active_watchers_count": len(tm.watchers),
		"loaded_tenant_ids": tm.getLoadedTenantsUnsafe(),
		"config_directory": tm.configDirectory,
	}
}

// getLoadedTenantsUnsafe returns list of currently loaded tenant IDs without locking
// NOTE: This method assumes the caller already holds the read mutex
func (tm *TenantManager) getLoadedTenantsUnsafe() []string {
	tenants := make([]string, 0, len(tm.tenantConfigs))
	for tenantID := range tm.tenantConfigs {
		tenants = append(tenants, tenantID)
	}
	return tenants
}

// TenantAwareConfig wraps the base Config to provide tenant-aware configuration access
type TenantAwareConfig struct {
	*Config
	tenantManager *TenantManager
}

// NewTenantAwareConfig creates a new tenant-aware configuration wrapper
func NewTenantAwareConfig(baseConfig *Config, configDirectory string) *TenantAwareConfig {
	tenantManager := NewTenantManager(baseConfig, configDirectory, baseConfig.l)
	return &TenantAwareConfig{
		Config:        baseConfig,
		tenantManager: tenantManager,
	}
}

// NewTenantAwareConfigWithManager creates a new tenant-aware configuration wrapper with an existing manager
func NewTenantAwareConfigWithManager(baseConfig *Config, tenantManager *TenantManager) *TenantAwareConfig {
	return &TenantAwareConfig{
		Config:        baseConfig,
		tenantManager: tenantManager,
	}
}

// GetProvider returns the appropriate configuration provider based on tenant context
func (tac *TenantAwareConfig) GetProvider(ctx context.Context) *configx.Provider {
	// Extract tenant ID from context
	tenantID := "default"
	if value := ctx.Value(TenantContextKey("tenant_id")); value != nil {
		if tid, ok := value.(string); ok {
			tenantID = tid
		}
	}
	return tac.tenantManager.GetTenantConfig(ctx, tenantID)
}

// GetTenantManager returns the underlying tenant manager
func (tac *TenantAwareConfig) GetTenantManager() *TenantManager {
	return tac.tenantManager
}

// SetTenantManager sets the tenant manager for this configuration
func (tac *TenantAwareConfig) SetTenantManager(tm *TenantManager) {
	tac.tenantManager = tm
}

// GetConfigDirectory returns the configuration directory from the tenant manager
func (tm *TenantManager) GetConfigDirectory() string {
	return tm.configDirectory
}

// Shutdown gracefully shuts down all watchers
func (tm *TenantManager) Shutdown() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for tenantID, cleanup := range tm.watchers {
		cleanup()
		tm.logger.WithField("tenant_id", tenantID).
			Debug("Stopped file watcher for tenant")
	}

	tm.watchers = make(map[string]func())
}
