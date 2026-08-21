// Comptoir, gestion de boutique hors ligne.
//
// Le programme assemble ici les trois couches : le stockage (fichiers JSON
// locaux), les services métier, et l'interface servie par Wails. Aucune des
// deux premières ne connaît Wails ; elles restent testables et réutilisables.
package main

import (
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"comptoir/internal/auth"
	"comptoir/internal/brand"
	"comptoir/internal/services"
	"comptoir/internal/storage"
)

//go:embed all:frontend/dist
var assets embed.FS

// version est renseignée à la compilation :
// go build -ldflags "-X main.version=1.0.0"
var version = "dev"

func main() {
	if err := run(); err != nil {
		// Sans fenêtre, il ne reste que la sortie standard et un code d'erreur.
		fmt.Fprintf(os.Stderr, "Comptoir n'a pas pu démarrer : %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// L'identité visuelle de l'auteur est vérifiée avant tout le reste. Un
	// échec n'empêche pas la boutique de travailler, priver un commerçant de
	// sa caisse serait disproportionné, mais il est consigné, et l'interface
	// le signale. Voir internal/brand et l'article 3 de la licence.
	brandErr := brand.Verify()

	dir, err := storage.DataDir()
	if err != nil {
		return err
	}
	db, err := storage.Open(dir)
	if err != nil {
		return err
	}
	if err := storage.Migrate(db); err != nil {
		return err
	}

	settings := db.Settings()
	sec := auth.New(db.Users, settings.SessionTimeoutMin)
	logger := newLogger(dir)
	if brandErr != nil {
		logger.Error(brandErr.Error())
	}
	services.Version = version

	// Reprise d'un accès administrateur perdu, si le fichier de demande est
	// présent. Avant toute session : c'est justement parce qu'aucune ne peut
	// s'ouvrir que la procédure existe.
	if reprise, err := services.RepriseAdministrateur(db, sec); err != nil {
		logger.Error("reprise d'accès : " + err.Error())
	} else if reprise.Effectuee {
		logger.Info(fmt.Sprintf(
			"reprise d'accès effectuée pour le compte « %s », mot de passe provisoire dans %s",
			reprise.Compte, reprise.Fichier))
	}

	var (
		session   = services.NewSession(db, sec)
		catalog   = services.NewCatalog(db, sec)
		stock     = services.NewStock(db, sec)
		sales     = services.NewSales(db, sec)
		creances  = services.NewCreances(db, sec)
		purchases = services.NewPurchases(db, sec)
		expenses  = services.NewExpenses(db, sec)
		reports   = services.NewReports(db, sec)
		documents = services.NewDocuments(db, sec)
		export    = services.NewExport(db, sec)
		users     = services.NewUsers(db, sec)
		config    = services.NewConfig(db, sec)
		backups   = services.NewBackups(db, sec)
	)
	app := NewApp(db, sec)

	return wails.Run(&options.App{
		Title:  "Comptoir",
		Width:  1280,
		Height: 820,
		// En dessous, les tableaux de la page Ventes ne tiennent plus.
		MinWidth:         1024,
		MinHeight:        680,
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		BackgroundColour: &options.RGBA{R: 249, G: 250, B: 249, A: 1},
		Bind: []any{
			app,
			session, catalog, stock, sales, creances, purchases, expenses,
			reports, documents, export, users, config, backups,
		},
		Mac: &mac.Options{
			// Barre de titre standard : les boutons fermer, réduire et
			// agrandir sont ceux du système, à la place où l'utilisateur les
			// cherche. Une barre dessinée par l'application se comporte
			// toujours un peu différemment de toutes les autres fenêtres.
			TitleBar: mac.TitleBarDefault(),
			About: &mac.AboutInfo{
				Title:   "Comptoir " + version,
				Message: "Gestion de boutique hors ligne.\nToutes les données restent sur ce poste.",
			},
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
		Logger: logger,
	})
}

// newLogger écrit le journal technique à côté des données plutôt qu'à la
// console : l'application est lancée par un raccourci, personne ne verra jamais
// un terminal.
func newLogger(dir string) *fileLogger {
	f, err := os.OpenFile(dir+"/comptoir.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		log.Printf("journal indisponible : %v", err)
		return &fileLogger{}
	}
	return &fileLogger{file: f}
}
