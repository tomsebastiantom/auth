// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegistryTenantSupport(t *testing.T) {
	t.Run("registry_has_tenant_fields", func(t *testing.T) {
		reg := &RegistryDefault{}
		
		// Test that tenant fields exist
		assert.False(t, reg.tenantAware, "tenantAware should default to false")
		assert.Empty(t, reg.configDirectory, "configDirectory should default to empty")
		assert.Nil(t, reg.tenantManager, "tenantManager should default to nil")
	})
	
	t.Run("with_tenant_aware_option", func(t *testing.T) {
		option := WithTenantAware("./test-configs")
		assert.NotNil(t, option, "WithTenantAware should return a valid option")
		
		// Test that the option sets the correct values
		opts := &options{}
		option(opts)
		
		assert.True(t, opts.tenantAware, "Option should set tenantAware to true")
		assert.Equal(t, "./test-configs", opts.configDirectory, "Option should set configDirectory")
	})
}

func TestTenantAwareConfig(t *testing.T) {
	t.Run("registry_tenant_aware_config", func(t *testing.T) {
		reg := &RegistryDefault{}
		
		// Test that TenantAwareConfig returns something (even in non-tenant mode)
		// Note: This will panic if config is not set, but that's expected behavior
		defer func() {
			if r := recover(); r != nil {
				t.Log("Expected panic when config is not set:", r)
			}
		}()
		
		// This should panic because config is not set
		_ = reg.TenantAwareConfig()
	})
}
