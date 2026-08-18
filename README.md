# Comptoir

Logiciel de gestion pour une boutique : stock, ventes, achats, charges,
rapports et factures PDF. Application de bureau, **entièrement hors ligne** :
aucune donnée ne quitte le poste, aucun compte à créer, aucun abonnement.

Pensé pour un commerce de détail en zone UEMOA — franc CFA sans décimale,
factures en français, TVA à 18 % par défaut — mais la monnaie, le taux de taxe
et les mentions légales se règlent dans les paramètres.

---

## Ce que fait le logiciel

**Catalogue et stock.** Articles, catégories, seuils d'alerte, emplacements,
suivi par numéro de série. Le stock défectueux est compté séparément et n'est
jamais vendable. Un catalogue existant s'importe depuis un tableur : les
colonnes sont reconnues par leur intitulé, et l'aperçu montre ligne par ligne
ce qui sera créé, mis à jour ou écarté — avant d'écrire quoi que ce soit.

**Ventes.** Devis, factures, règlements partiels, annulations. Le stock est
déduit à l'émission. Chaque facture porte sa marge, calculée sur le coût moyen
du jour de la vente. La saisie se fait au clavier : on cherche ou on scanne, les
flèches choisissent, Entrée ajoute — une douchette code-barres fonctionne sans
réglage.

**Achats.** Réceptions fournisseur avec frais annexes répartis au prorata. Le
coût moyen pondéré (CUMP) est recalculé à chaque entrée ; une marge cible peut
réajuster les prix de vente automatiquement.

**Mouvements.** Retours clients et fournisseurs, déclarations de défaut,
réparations, rebuts, corrections d'inventaire. **Aucune quantité ne change sans
mouvement daté et signé** — c'est la règle qui rend l'inventaire vérifiable.

**Charges et rapports.** Charges d'exploitation par rubrique, compte de
résultat, situation patrimoniale simplifiée, statistiques par catégorie,
client, vendeur et jour de la semaine. Export CSV pour tableur, PDF pour
l'impression.

**Comptes.** Trois rôles — administrateur, gérant, vendeur — et un journal
d'audit en ajout seul. Un vendeur ne voit ni les prix d'achat, ni les marges,
ni les charges : le filtrage est appliqué côté Go, pas seulement à l'écran.
Chacun change son propre mot de passe depuis sa fiche ; un mot de passe
réinitialisé par un administrateur n'ouvre rien tant qu'il n'a pas été remplacé.

---

## Installation

### Utilisation

Téléchargez l'exécutable et lancez-le. Au premier démarrage, vous créez le
compte administrateur ; le catalogue est amorcé avec des catégories courantes.

Les données sont écrites dans :

| Système | Emplacement |
|---|---|
| Windows | `%APPDATA%\Comptoir` |
| macOS | `~/Library/Application Support/Comptoir` |
| Linux | `~/.config/Comptoir` |

**Mode portable.** Placez un fichier vide nommé `portable.txt` à côté de
l'exécutable : les données vivent alors dans un dossier `data` voisin. Pratique
pour travailler depuis une clé USB.

### Compilation

Prérequis : [Go](https://go.dev) 1.23 ou plus, [Node.js](https://nodejs.org) 20
ou plus, et la ligne de commande Wails :

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Puis, à la racine du dépôt :

```sh
make dev      # lance l'application avec rechargement à chaud
make build    # produit l'exécutable dans build/bin
make test     # exécute la suite de tests Go
make check    # tests + vet + typage du frontend
```

`wails doctor` signale les dépendances système manquantes (WebView2 sous
Windows, `libgtk-3-dev` et `libwebkit2gtk-4.0-dev` sous Linux).

---

## Comment c'est fait

```
main.go, app.go       Assemblage et couche Wails : fenêtre, sélecteurs de
                      fichiers, cycle de vie. Rien de métier ici.
internal/models       Structures persistées. Tous les montants sont des
                      entiers en centièmes — jamais de flottant sur l'argent.
internal/storage      Persistance JSON : écriture atomique, verrou par
                      collection, transactions, sauvegardes ZIP, migrations.
internal/auth         Sessions locales, bcrypt, rôles et permissions.
internal/services     Logique métier. Chaque méthode exportée est un point
                      d'entrée de l'interface.
internal/pdfgen       Factures, bons d'entrée et rapports imprimables.
frontend/             Interface React + TypeScript, sans bibliothèque de
                      composants ni de graphes.
```

Trois décisions structurantes :

**Les montants sont des entiers.** 1 500 FCFA valent `150000`. Les divisions
sont arrondies au plus proche, jamais tronquées, pour que la somme des lignes
corresponde toujours au total imprimé.

**Les données sont du JSON lisible**, pas une base binaire. Un fichier corrompu
se répare dans un éditeur de texte ; une sauvegarde s'inspecte sans outil.
L'écriture est atomique — fichier temporaire, synchronisation disque, puis
renommage — donc jamais de fichier à moitié écrit, même en cas de coupure de
courant.

**Le contrôle d'accès est côté Go.** Masquer un bouton n'est pas une
protection ; refuser l'appel en est une. Les projections envoyées à l'interface
retirent les données de coût quand le rôle n'y a pas droit.

Plus de détail : [docs/COMPTABILITE.md](docs/COMPTABILITE.md) pour les règles de
calcul et leurs limites, [docs/SECURITE.md](docs/SECURITE.md) pour le modèle de
menace.

---

## Sauvegardes

Une archive ZIP complète est créée automatiquement au premier démarrage de
chaque journée, et les trente dernières sont conservées. Les sauvegardes
manuelles et la restauration se font dans **Paramètres → Sauvegardes**.

Une sauvegarde qui vit sur le même disque que les données ne protège pas d'une
panne de ce disque : copiez-en une sur une clé USB de temps en temps.

---

## Ce que le logiciel ne fait pas

Ces limites sont assumées, pas oubliées :

- pas de comptabilité en partie double ni d'états certifiés ;
- pas de suivi des échéances fournisseurs — la trésorerie affichée est une
  estimation, signalée comme telle ;
- pas de multi-boutique ni de synchronisation réseau : un poste, des données ;
- le coût moyen pondéré n'est pas recalculé à rebours lors de l'annulation d'un
  bon d'entrée ; il se réaligne à la réception suivante.

## Licence

Ce dépôt ne fixe pas encore de licence. Ajoutez-en une avant toute
redistribution.
