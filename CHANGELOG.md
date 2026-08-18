# Journal des versions

Le format suit [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/), et la
numérotation le [versionnage sémantique](https://semver.org/lang/fr/).

Les entrées décrivent ce qui change pour la personne qui tient la boutique,
pas la liste des fichiers modifiés.

## [Non publié]

Rien pour l'instant.

## [1.0.0] — 2026-08-18

Première version utilisable de bout en bout.

### Ajouté

**Premier démarrage**
- Assistant en sept étapes : présentation, compte administrateur, identité de
  l'entreprise, monnaie et taxes, catalogue de départ, sauvegardes,
  récapitulatif. Rien n'est écrit avant la validation finale ; on peut revenir
  en arrière à tout moment.
- Catégories de départ proposées et modifiables, jamais imposées.

**Catalogue et stock**
- Articles avec référence, code-barres, marque, modèle, unité, emplacement,
  garantie et suivi par numéro de série.
- Catégories avec code couleur, indicateurs de volume et de valeur.
- Seuils d'alerte, distinction du stock vendable et du stock défectueux.
- Import d'un catalogue depuis un tableur : colonnes reconnues par leur
  intitulé dans n'importe quel ordre, séparateur deviné, encodage hérité
  converti, et aperçu ligne à ligne avant la moindre écriture.
- Fiche de vie d'un article : tous ses mouvements, ses ventes, sa marge.

**Ventes**
- Devis, factures, règlements partiels, annulations avec motif obligatoire.
- Saisie au clavier de bout en bout ; une douchette code-barres fonctionne
  sans réglage, y compris lorsque la validation arrive avant les résultats de
  recherche.
- Remises par ligne et remise globale répartie au prorata.
- Taux de taxe par ligne, avec exonération explicite possible.
- Factures et devis imprimables, montant en toutes lettres.

**Achats**
- Bons d'entrée fournisseur avec frais annexes répartis au prorata.
- Recalcul du coût moyen pondéré à chaque réception, frais compris.
- Marge cible facultative, qui réajuste les prix de vente à la réception.
- Annulation d'un bon, refusée si la marchandise est déjà partie.

**Mouvements**
- Retours clients et fournisseurs, déclarations de défaut, réparations,
  rebuts, corrections d'inventaire.
- Journal complet, filtrable, exportable, avec référence unique par mouvement.

**Charges et rapports**
- Charges d'exploitation par rubrique, avec répartition et part de chacune.
- Tableau de bord : ventes du jour et du mois, marge, impayés, courbes sur
  30 jours et 12 mois, meilleures ventes, stock à surveiller.
- Rapport de ventes par jour, semaine, mois, trimestre, semestre ou année.
- Compte de résultat, situation patrimoniale simplifiée, statistiques par
  catégorie, client, vendeur, mode de règlement et jour de la semaine.
- Articles dormants : le stock qui ne tourne pas, et ce qu'il immobilise.
- Export CSV lisible par Excel francophone, PDF pour l'impression.

**Comptes et sécurité**
- Trois rôles : administrateur, gérant, vendeur.
- Un vendeur ne reçoit ni prix d'achat, ni marge, ni valorisation de stock :
  le filtrage est appliqué dans les services, pas seulement à l'écran.
- Fiche « Mon compte » : chacun change son propre mot de passe.
- Mot de passe provisoire généré à la réinitialisation, à changer à la
  première connexion — d'ici là, le compte n'ouvre rien.
- Journal d'audit en ajout seul, filtrable.
- Verrouillage après cinq tentatives, expiration de session réglable.

**Sauvegardes**
- Archive complète créée automatiquement au premier démarrage de chaque
  journée, avec rétention réglable.
- Sauvegarde manuelle, restauration depuis la liste ou depuis un fichier
  externe, sauvegarde de sécurité prise avant toute restauration.

### Garanties techniques

- Montants en entiers, jamais en flottants ; arrondi au plus proche avec
  report du reliquat, pour que la somme des lignes égale toujours le total
  imprimé.
- Aucune quantité de stock ne change sans mouvement daté et signé.
- Application d'un document en une transaction par collection : tout passe ou
  rien ne change.
- Écriture atomique des fichiers : une coupure de courant ne laisse jamais de
  fichier à moitié écrit.
- Refus d'ouvrir des données créées par une version plus récente.
- Protection contre les archives piégées à la restauration.

[Non publié]: https://github.com/siskoai/stock/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/siskoai/stock/releases/tag/v1.0.0
