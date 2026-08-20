package main

// Ouverture d'un dossier dans le gestionnaire de fichiers du système.
//
// Ce code vit dans son propre fichier parce qu'il a besoin du paquet « runtime »
// de la bibliothèque standard, dont le nom entre en conflit avec celui de Wails
// utilisé partout ailleurs. Un fichier séparé évite un alias d'import que
// personne n'aurait envie de lire.

import (
	"fmt"
	"os/exec"
	"runtime"
)

// revelerDansExplorateur ouvre un dossier dans le gestionnaire de fichiers.
//
// Passer par une URL « file:// » ne suffit pas : selon le système, elle est
// confiée au navigateur, qui affiche une liste de fichiers dans un onglet au
// lieu d'ouvrir le Finder ou l'Explorateur, quand elle n'est pas simplement
// ignorée. Chaque système a sa commande, et c'est elle qu'il faut appeler.
func revelerDansExplorateur(chemin string) error {
	var commande string
	switch runtime.GOOS {
	case "darwin":
		commande = "open"
	case "windows":
		// explorer.exe rend un code de sortie non nul même lorsqu'il réussit :
		// son résultat n'est pas exploitable, on ne le lit donc pas.
		commande = "explorer"
	default:
		commande = "xdg-open"
	}
	cmd := exec.Command(commande, chemin)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("impossible d'ouvrir le dossier. Il se trouve dans %s", chemin)
	}
	// On n'attend pas la fin : le gestionnaire de fichiers reste ouvert, et
	// bloquer ici figerait l'application tant que sa fenêtre est affichée.
	go func() { _ = cmd.Wait() }()
	return nil
}
