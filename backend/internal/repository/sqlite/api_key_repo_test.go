package sqlite

import (
	"os"
	"testing"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func TestApiKeyDatetimeScanning(t *testing.T) {
	// Create a temporary database for testing
	dbPath := "/tmp/test_api_keys_datetime.db"
	defer os.Remove(dbPath)

	// Open database
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Ensure tables are created
	if err := db.EnsureTables(); err != nil {
		t.Fatalf("Failed to ensure tables: %v", err)
	}

	// Create a test API key
	apiKey := &domain.ApiKey{
		ID:       "test-key-001",
		Provider: "anthropic",
		KeyEnc:   "encrypted-key-data",
		Label:    "Test API Key",
	}

	// Insert the API key
	if err := db.CreateApiKey(apiKey); err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	// Test 1: Retrieve by provider (FindApiKeyByProvider)
	retrieved, err := db.FindApiKeyByProvider("anthropic")
	if err != nil {
		t.Fatalf("Failed to find API key by provider: %v", err)
	}

	if retrieved.ID != "test-key-001" {
		t.Errorf("Expected ID 'test-key-001', got %s", retrieved.ID)
	}

	// Verify datetime fields are populated
	if retrieved.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
	if retrieved.UpdatedAt.IsZero() {
		t.Error("UpdatedAt is zero")
	}

	// Verify datetime is recent (within last minute)
	now := time.Now()
	if now.Sub(retrieved.CreatedAt) > time.Minute {
		t.Error("CreatedAt is not recent")
	}

	// Test 2: Retrieve by ID (FindApiKey)
	retrieved2, err := db.FindApiKey("test-key-001")
	if err != nil {
		t.Fatalf("Failed to find API key by ID: %v", err)
	}

	if retrieved2.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero (FindApiKey)")
	}

	// Test 3: List all API keys (ListApiKeys)
	keys, err := db.ListApiKeys()
	if err != nil {
		t.Fatalf("Failed to list API keys: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("Expected 1 key, got %d", len(keys))
	}

	if keys[0].CreatedAt.IsZero() {
		t.Error("CreatedAt is zero (ListApiKeys)")
	}

	// Test 4: Update a key (delete + create)
	apiKey2 := &domain.ApiKey{
		ID:       "test-key-001",
		Provider: "anthropic",
		KeyEnc:   "new-encrypted-key-data",
		Label:    "Updated Test API Key",
	}
	if err := db.DeleteApiKey("test-key-001"); err != nil {
		t.Fatalf("Failed to delete API key: %v", err)
	}
	if err := db.CreateApiKey(apiKey2); err != nil {
		t.Fatalf("Failed to create updated API key: %v", err)
	}

	retrieved3, err := db.FindApiKey("test-key-001")
	if err != nil {
		t.Fatalf("Failed to find updated API key: %v", err)
	}
	if retrieved3.KeyEnc != "new-encrypted-key-data" {
		t.Error("Key was not updated")
	}
	if retrieved3.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero after update")
	}

	t.Log("All tests passed!")
}
