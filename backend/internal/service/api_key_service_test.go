package service

import (
	"os"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

func TestApiKeyServiceSet(t *testing.T) {
	// Create a temporary database for testing
	dbPath := "/tmp/test_api_key_service.db"
	defer os.Remove(dbPath)

	// Open database and ensure tables
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.EnsureTables(); err != nil {
		t.Fatalf("Failed to ensure tables: %v", err)
	}

	// Create service
	service := NewApiKeyService(db, "test-jwt-secret")

	// Test 1: Set an API key for a new provider
	t.Log("Test 1: Setting a new API key for anthropic provider")
	resp1, err := service.Set("anthropic", "sk-test-key-123", "My Test Key")
	if err != nil {
		t.Fatalf("Failed to set API key: %v", err)
	}

	if resp1.Provider != "anthropic" {
		t.Errorf("Expected provider 'anthropic', got %s", resp1.Provider)
	}
	if resp1.Label != "My Test Key" {
		t.Errorf("Expected label 'My Test Key', got %s", resp1.Label)
	}
	if resp1.CreatedAt.IsZero() {
		t.Error("CreatedAt should be populated")
	}
	if resp1.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be populated")
	}
	t.Logf("✓ Created key with ID: %s", resp1.ID)

	// Test 2: List all keys
	t.Log("Test 2: Listing all API keys")
	keys, err := service.List()
	if err != nil {
		t.Fatalf("Failed to list API keys: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("Expected 1 key, got %d", len(keys))
	}
	if keys[0].CreatedAt.IsZero() {
		t.Error("CreatedAt should be populated in list")
	}
	t.Logf("✓ Listed %d key(s)", len(keys))

	// Test 3: Get a specific key
	t.Log("Test 3: Getting a specific API key")
	key, err := service.Get(resp1.ID)
	if err != nil {
		t.Fatalf("Failed to get API key: %v", err)
	}
	if key.ID != resp1.ID {
		t.Errorf("Expected ID %s, got %s", resp1.ID, key.ID)
	}
	if key.CreatedAt.IsZero() {
		t.Error("CreatedAt should be populated in get")
	}
	t.Logf("✓ Got key with ID: %s", key.ID)

	// Test 4: Replace existing key for same provider
	t.Log("Test 4: Replacing the existing API key for anthropic provider")
	resp2, err := service.Set("anthropic", "sk-test-key-456", "Updated Key")
	if err != nil {
		t.Fatalf("Failed to set API key (update): %v", err)
	}

	if resp2.Provider != "anthropic" {
		t.Errorf("Expected provider 'anthropic', got %s", resp2.Provider)
	}
	if resp2.ID != resp1.ID {
		t.Errorf("Expected ID to remain %s, got %s", resp1.ID, resp2.ID)
	}
	if resp2.Label != "Updated Key" {
		t.Errorf("Expected label 'Updated Key', got %s", resp2.Label)
	}
	if resp2.CreatedAt.IsZero() {
		t.Error("CreatedAt should be populated after update")
	}
	t.Logf("✓ Updated key, new label: %s", resp2.Label)

	// Test 5: Verify only one key remains
	t.Log("Test 5: Verifying only one key exists after update")
	keys2, err := service.List()
	if err != nil {
		t.Fatalf("Failed to list API keys after update: %v", err)
	}
	if len(keys2) != 1 {
		t.Errorf("Expected 1 key after update, got %d", len(keys2))
	}
	t.Logf("✓ Confirmed single key after update")

	// Test 6: Verify key is retrievable by provider
	t.Log("Test 6: Verifying key can be retrieved after update")
	retrieved, err := db.FindApiKeyByProvider("anthropic")
	if err != nil {
		t.Fatalf("Failed to find key by provider: %v", err)
	}
	if retrieved.ID != resp1.ID {
		t.Errorf("Expected ID %s, got %s", resp1.ID, retrieved.ID)
	}
	if retrieved.CreatedAt.IsZero() {
		t.Error("CreatedAt should be populated in FindApiKeyByProvider")
	}
	t.Logf("✓ Key successfully retrieved by provider")

	t.Log("✓ All service-level tests passed!")
}
