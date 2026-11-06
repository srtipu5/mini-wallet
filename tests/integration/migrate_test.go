package tests

import (
	"os"
	"strings"
	"testing"

	"github.com/srtipu5/mini-wallet/internal/infrastructure/postgres"
)

// Migrations accept both URL schemes and are safe to run again on an
// up-to-date database (which is what happens on every API restart).
func TestMigrate_IsRepeatableWithBothURLSchemes(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	for _, u := range []string{
		url,
		strings.Replace(url, "postgres://", "postgresql://", 1),
	} {
		if err := postgres.Migrate(u); err != nil {
			t.Fatalf("Migrate(%s): %v", u[:strings.Index(u, "://")], err)
		}
	}
}
