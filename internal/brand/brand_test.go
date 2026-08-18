package brand

import (
	"bytes"
	"strings"
	"testing"
)

// L'œuvre intégrée doit être exactement celle déposée. Cet échec ne se
// « corrige » pas en mettant la constante à jour : il signale que le logo a
// changé, ce que la licence interdit.
func TestLogoIntact(t *testing.T) {
	if err := Verify(); err != nil {
		t.Fatalf("intégrité de la marque : %v", err)
	}
}

func TestLogoEstUnPNGNonVide(t *testing.T) {
	data := Logo()
	if len(data) < 1000 {
		t.Fatalf("logo suspect : %d octets", len(data))
	}
	if !bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G'}) {
		t.Error("le logo intégré n'est pas un PNG")
	}
}

// Logo renvoie une copie : un appelant ne doit pas pouvoir altérer l'original
// détenu par le paquet.
func TestLogoRenvoieUneCopie(t *testing.T) {
	first := Logo()
	for i := range first {
		first[i] = 0
	}
	if err := Verify(); err != nil {
		t.Fatalf("l'original a été altéré par un appelant : %v", err)
	}
	if bytes.Equal(first, Logo()) {
		t.Error("Logo() partage sa mémoire avec l'original")
	}
}

func TestAttribution(t *testing.T) {
	a := Current()
	if !a.Intact || a.Alert != "" {
		t.Errorf("attribution signalée altérée : %+v", a)
	}
	if a.Author != "SISKO" {
		t.Errorf("auteur = %q, attendu SISKO", a.Author)
	}
	if !strings.HasPrefix(a.LogoDataURL, "data:image/png;base64,") {
		t.Error("le logo n'est pas transmis sous une forme affichable")
	}
	if !strings.Contains(a.Notice, "SISKO") {
		t.Errorf("la mention ne nomme pas l'auteur : %q", a.Notice)
	}
}
