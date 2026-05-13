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
