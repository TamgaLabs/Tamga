package service

import (
	"os"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

func countDefaults(t *testing.T, svc *AgentProviderService) (count int, defaultID string) {
	t.Helper()
	providers, err := svc.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, p := range providers {
		if p.IsDefault {
			count++
			defaultID = p.ID
		}
	}
	return count, defaultID
}

func TestAgentProviderServiceDefaultExclusivity(t *testing.T) {
	dbPath := "/tmp/test_agent_provider_service.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := NewAgentProviderService(db)

	// Migration seeds builtin-opencode as the sole default.
	count, defaultID := countDefaults(t, svc)
	if count != 1 || defaultID != "builtin-opencode" {
		t.Fatalf("expected exactly 1 default (builtin-opencode) after seed, got count=%d id=%q", count, defaultID)
	}

	// Creating a new provider with is_default:true must clear the
	// previous default rather than resulting in two default rows.
	newProvider := &domain.AgentProvider{
		Name:      "Custom Provider",
		Type:      domain.ProviderTypeDocker,
		Image:     "tamga-agent-custom",
		IsDefault: true,
	}
	if err := svc.Create(newProvider); err != nil {
		t.Fatalf("create: %v", err)
	}

	count, defaultID = countDefaults(t, svc)
	if count != 1 {
		t.Fatalf("expected exactly 1 default after create with is_default:true, got %d", count)
	}
	if defaultID != newProvider.ID {
		t.Fatalf("expected new provider %q to be the default, got %q", newProvider.ID, defaultID)
	}

	old, err := svc.Get("builtin-opencode")
	if err != nil {
		t.Fatalf("get old default: %v", err)
	}
	if old.IsDefault {
		t.Fatalf("expected previous default (builtin-opencode) to have is_default cleared")
	}

	// The old default is no longer flagged as default, so Update's
	// "cannot modify default provider" guard should no longer block it -
	// but updating a *third*, non-default provider to is_default:true
	// should flip the flag again without leaving two defaults.
	third := &domain.AgentProvider{
		Name:  "Third Provider",
		Type:  domain.ProviderTypeDocker,
		Image: "tamga-agent-third",
	}
	if err := svc.Create(third); err != nil {
		t.Fatalf("create third: %v", err)
	}

	third.Name = "Third Provider Renamed"
	third.Type = domain.ProviderTypeDocker
	third.Image = "tamga-agent-third"
	third.IsDefault = true
	if err := svc.Update(third); err != nil {
		t.Fatalf("update third to default: %v", err)
	}

	count, defaultID = countDefaults(t, svc)
	if count != 1 {
		t.Fatalf("expected exactly 1 default after update with is_default:true, got %d", count)
	}
	if defaultID != third.ID {
		t.Fatalf("expected third provider %q to be the default, got %q", third.ID, defaultID)
	}

	prevDefault, err := svc.Get(newProvider.ID)
	if err != nil {
		t.Fatalf("get previous default: %v", err)
	}
	if prevDefault.IsDefault {
		t.Fatalf("expected previously-default provider %q to have is_default cleared", newProvider.ID)
	}

	// ResolveProvider with no explicit id should now resolve to the
	// current default, unambiguously.
	resolved, err := svc.ResolveProvider("")
	if err != nil {
		t.Fatalf("resolve default provider: %v", err)
	}
	if resolved.ID != third.ID {
		t.Fatalf("expected ResolveProvider(\"\") to return %q, got %q", third.ID, resolved.ID)
	}
}
