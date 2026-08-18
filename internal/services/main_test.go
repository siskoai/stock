package services

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"comptoir/internal/auth"
)

// TestMain abaisse le coût bcrypt pour la durée des tests : chaque suite ouvre
// une session, et le hachage dominerait autrement le temps d'exécution.
func TestMain(m *testing.M) {
	auth.Cost = bcrypt.MinCost
	os.Exit(m.Run())
}
