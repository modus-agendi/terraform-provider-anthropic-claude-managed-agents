package provider

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/andasv/terraform-provider-claude-managed-agents/internal/client"
)

// testResourcePrefix is the name prefix every live-mode test resource must
// start with. The sweeper matches this prefix; the random suffix in
// randomTestName guarantees collisions are unlikely across concurrent runs.
const testResourcePrefix = "tf-acc-test-"

// sweepAgeThreshold is how old an agent must be before the sweeper will
// archive it. Keeps the sweeper from racing with active live tests.
const sweepAgeThreshold = 1 * time.Hour

// TestMain wires the terraform-plugin-testing sweeper flags (-sweep,
// -sweep-run, -sweep-allow-failures) into go test. Without this entrypoint,
// running `go test -sweep=foo` would just be a no-op.
func TestMain(m *testing.M) {
	resource.TestMain(m)
}

func init() {
	resource.AddTestSweepers("claude-managed-agents_agent", &resource.Sweeper{
		Name: "claude-managed-agents_agent",
		F:    sweepAgents,
	})
	resource.AddTestSweepers("claude-managed-agents_environment", &resource.Sweeper{
		Name: "claude-managed-agents_environment",
		F:    sweepEnvironments,
	})
	resource.AddTestSweepers("claude-managed-agents_memory_store", &resource.Sweeper{
		Name: "claude-managed-agents_memory_store",
		F:    sweepMemoryStores,
	})
	resource.AddTestSweepers("claude-managed-agents_vault", &resource.Sweeper{
		Name: "claude-managed-agents_vault",
		F:    sweepVaults,
	})
	resource.AddTestSweepers("claude-managed-agents_skill", &resource.Sweeper{
		Name: "claude-managed-agents_skill",
		F:    sweepSkills,
	})
}

// sweepAgents archives any agent whose name starts with `tf-acc-test-` and
// was created more than sweepAgeThreshold ago. The region argument is
// ignored — Anthropic's API is global.
func sweepAgents(_ string) error {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Println("[INFO] ANTHROPIC_API_KEY not set; sweeper is a no-op")
		return nil
	}

	c, err := client.New(client.Config{
		APIKey:    apiKey,
		UserAgent: "terraform-provider-claude-managed-agents/sweeper",
	})
	if err != nil {
		return fmt.Errorf("sweeper: build client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var (
		cursor       string
		swept        int
		skippedYoung int
	)
	for {
		page, err := c.ListAgents(ctx, client.ListAgentsParams{Limit: 100, AfterID: cursor})
		if err != nil {
			return fmt.Errorf("sweeper: list agents: %w", err)
		}
		for _, a := range page.Data {
			if !strings.HasPrefix(a.Name, testResourcePrefix) {
				continue
			}
			if time.Since(a.CreatedAt) < sweepAgeThreshold {
				skippedYoung++
				continue
			}
			if err := c.ArchiveAgent(ctx, a.ID); err != nil && !client.IsNotFound(err) {
				return fmt.Errorf("sweeper: archive %s: %w", a.ID, err)
			}
			log.Printf("[INFO] swept agent %s (%s)", a.ID, a.Name)
			swept++
		}
		if !page.HasMore {
			break
		}
		cursor = page.LastID
	}

	log.Printf("[INFO] sweeper finished: archived=%d skipped_young=%d", swept, skippedYoung)
	return nil
}

// sweepEnvironments deletes any environment whose name starts with
// tf-acc-test- and was created more than sweepAgeThreshold ago. Delete is
// tried first; on 409 we fall back to archive so live tests don't get stuck
// holding the slot.
func sweepEnvironments(_ string) error {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Println("[INFO] ANTHROPIC_API_KEY not set; environment sweeper is a no-op")
		return nil
	}

	c, err := client.New(client.Config{
		APIKey:    apiKey,
		UserAgent: "terraform-provider-claude-managed-agents/sweeper",
	})
	if err != nil {
		return fmt.Errorf("env sweeper: build client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var (
		cursor       string
		deleted      int
		archived     int
		skippedYoung int
	)
	for {
		page, err := c.ListEnvironments(ctx, client.ListEnvironmentsParams{Limit: 100, AfterID: cursor, IncludeArchived: true})
		if err != nil {
			return fmt.Errorf("env sweeper: list: %w", err)
		}
		for _, e := range page.Data {
			if !strings.HasPrefix(e.Name, testResourcePrefix) {
				continue
			}
			if time.Since(e.CreatedAt) < sweepAgeThreshold {
				skippedYoung++
				continue
			}
			if e.ArchivedAt != nil {
				continue
			}
			if err := c.DeleteEnvironment(ctx, e.ID); err == nil {
				log.Printf("[INFO] swept (deleted) environment %s (%s)", e.ID, e.Name)
				deleted++
				continue
			} else if !client.IsConflict(err) && !client.IsNotFound(err) {
				return fmt.Errorf("env sweeper: delete %s: %w", e.ID, err)
			}
			if err := c.ArchiveEnvironment(ctx, e.ID); err != nil && !client.IsNotFound(err) {
				return fmt.Errorf("env sweeper: archive %s: %w", e.ID, err)
			}
			log.Printf("[INFO] swept (archived) environment %s (%s)", e.ID, e.Name)
			archived++
		}
		if !page.HasMore {
			break
		}
		cursor = page.LastID
	}

	log.Printf("[INFO] env sweeper finished: deleted=%d archived=%d skipped_young=%d", deleted, archived, skippedYoung)
	return nil
}

// sweepMemoryStores archives any memory store whose name starts with
// tf-acc-test- and was created more than sweepAgeThreshold ago. Archive
// is preferred over delete to preserve audit trails for shared accounts.
func sweepMemoryStores(_ string) error {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Println("[INFO] ANTHROPIC_API_KEY not set; memory store sweeper is a no-op")
		return nil
	}

	c, err := client.New(client.Config{
		APIKey:    apiKey,
		UserAgent: "terraform-provider-claude-managed-agents/sweeper",
	})
	if err != nil {
		return fmt.Errorf("memory store sweeper: build client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var (
		cursor       string
		swept        int
		skippedYoung int
	)
	for {
		page, err := c.ListMemoryStores(ctx, client.ListMemoryStoresParams{Limit: 100, AfterID: cursor})
		if err != nil {
			return fmt.Errorf("memory store sweeper: list: %w", err)
		}
		for _, s := range page.Data {
			if !strings.HasPrefix(s.Name, testResourcePrefix) {
				continue
			}
			if time.Since(s.CreatedAt) < sweepAgeThreshold {
				skippedYoung++
				continue
			}
			if err := c.ArchiveMemoryStore(ctx, s.ID); err != nil && !client.IsNotFound(err) {
				return fmt.Errorf("memory store sweeper: archive %s: %w", s.ID, err)
			}
			log.Printf("[INFO] swept memory_store %s (%s)", s.ID, s.Name)
			swept++
		}
		if !page.HasMore {
			break
		}
		cursor = page.LastID
	}

	log.Printf("[INFO] memory store sweeper finished: archived=%d skipped_young=%d", swept, skippedYoung)
	return nil
}

// sweepVaults archives any vault whose display_name starts with
// tf-acc-test- and was created more than sweepAgeThreshold ago. Archiving
// the vault cascades through credentials server-side, so we don't need a
// separate credential sweeper.
func sweepVaults(_ string) error {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Println("[INFO] ANTHROPIC_API_KEY not set; vault sweeper is a no-op")
		return nil
	}

	c, err := client.New(client.Config{
		APIKey:    apiKey,
		UserAgent: "terraform-provider-claude-managed-agents/sweeper",
	})
	if err != nil {
		return fmt.Errorf("vault sweeper: build client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var (
		cursor       string
		swept        int
		skippedYoung int
	)
	for {
		page, err := c.ListVaults(ctx, client.ListVaultsParams{Limit: 100, AfterID: cursor})
		if err != nil {
			return fmt.Errorf("vault sweeper: list: %w", err)
		}
		for _, v := range page.Data {
			if !strings.HasPrefix(v.DisplayName, testResourcePrefix) {
				continue
			}
			if time.Since(v.CreatedAt) < sweepAgeThreshold {
				skippedYoung++
				continue
			}
			if err := c.ArchiveVault(ctx, v.ID); err != nil && !client.IsNotFound(err) {
				return fmt.Errorf("vault sweeper: archive %s: %w", v.ID, err)
			}
			log.Printf("[INFO] swept vault %s (%s)", v.ID, v.DisplayName)
			swept++
		}
		if !page.HasMore {
			break
		}
		cursor = page.LastID
	}

	log.Printf("[INFO] vault sweeper finished: archived=%d skipped_young=%d", swept, skippedYoung)
	return nil
}

// sweepSkills deletes any custom skill whose display_title starts with
// tf-acc-test- and was created more than sweepAgeThreshold ago. The
// upstream API rejects deleting a skill while versions remain, so we
// cascade through versions first. Prebuilt skills (source=anthropic) are
// never touched.
func sweepSkills(_ string) error {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Println("[INFO] ANTHROPIC_API_KEY not set; skill sweeper is a no-op")
		return nil
	}

	c, err := client.New(client.Config{
		APIKey:    apiKey,
		UserAgent: "terraform-provider-claude-managed-agents/sweeper",
	})
	if err != nil {
		return fmt.Errorf("skill sweeper: build client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var (
		cursor       string
		swept        int
		skippedYoung int
	)
	for {
		page, err := c.ListSkills(ctx, client.ListSkillsParams{Limit: 100, AfterID: cursor, Source: "custom"})
		if err != nil {
			return fmt.Errorf("skill sweeper: list: %w", err)
		}
		for _, s := range page.Data {
			if s.Source != "custom" {
				continue
			}
			if !strings.HasPrefix(s.DisplayTitle, testResourcePrefix) {
				continue
			}
			if time.Since(s.CreatedAt) < sweepAgeThreshold {
				skippedYoung++
				continue
			}
			// Cascade delete versions before deleting the skill.
			versions, err := c.ListSkillVersions(ctx, s.ID)
			if err != nil && !client.IsNotFound(err) {
				return fmt.Errorf("skill sweeper: list versions %s: %w", s.ID, err)
			}
			if versions != nil {
				for _, v := range versions.Data {
					if err := c.DeleteSkillVersion(ctx, s.ID, v.Version); err != nil && !client.IsNotFound(err) {
						return fmt.Errorf("skill sweeper: delete version %s/%s: %w", s.ID, v.Version, err)
					}
				}
			}
			if err := c.DeleteSkill(ctx, s.ID); err != nil && !client.IsNotFound(err) {
				return fmt.Errorf("skill sweeper: delete %s: %w", s.ID, err)
			}
			log.Printf("[INFO] swept skill %s (%s)", s.ID, s.DisplayTitle)
			swept++
		}
		if !page.HasMore {
			break
		}
		cursor = page.LastID
	}

	log.Printf("[INFO] skill sweeper finished: deleted=%d skipped_young=%d", swept, skippedYoung)
	return nil
}
