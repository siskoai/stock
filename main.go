// Comptoir — gestion de boutique hors ligne.
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
	services.Version = version

	var (
		session   = services.NewSession(db, sec)
		catalog   = services.NewCatalog(db, sec)
		stock     = services.NewStock(db, sec)
		sales     = services.NewSales(db, sec)
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
			session, catalog, stock, sales, purchases, expenses,
			reports, documents, export, users, config, backups,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title:   "Comptoir " + version,
				Message: "Gestion de boutique hors ligne.\nToutes les données restent sur ce poste.",
			},
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
		Logger: newLogger(dir),
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
