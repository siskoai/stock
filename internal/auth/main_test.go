package auth

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestMain abaisse le coût bcrypt pour la durée des tests. Le comportement
// vérifié est le même ; seul le temps passé à hacher change.
func TestMain(m *testing.M) {
	Cost = bcrypt.MinCost
	os.Exit(m.Run())
}
