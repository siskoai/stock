# Journal des versions

Le format suit [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/), et la
numérotation le [versionnage sémantique](https://semver.org/lang/fr/).

Les entrées décrivent ce qui change pour la personne qui tient la boutique,
pas la liste des fichiers modifiés.

## [Non publié]

Rien pour l'instant.

## [1.3.0] (2026-08-21)

### Corrigé

- **Une vente à crédit s'enregistrait comme réglée.** Le champ « montant reçu »
  suivait le total tant qu'on n'y touchait pas, ce qui est juste pour un
  règlement comptant et faux pour un crédit : la facture sortait soldée, et la
  dette n'existait nulle part. Choisir « Crédit » met désormais ce champ à zéro,
  et le bouton d'enregistrement annonce ce qui reste dû plutôt que ce qui rentre
  en caisse.
- **Les onglets des rapports ne répondaient plus** quand la fenêtre se
  resserrait : le sélecteur de période recouvrait le dernier onglet, et le clic
  atterrissait sur le bouton au lieu de l'onglet. Corrigé avec les marges des
  rapports en 1.2.0, mais il fallait une version pour en profiter.
- **La colonne « Désignation » de l'état du stock était coupée.** Avec les
  colonnes de coût, il ne lui restait que 13 mm, soit sept caractères. La
  répartition des largeurs garantit maintenant un plancher aux colonnes libres,
  quitte à resserrer les colonnes chiffrées, et la troncature mesure le texte
  avec la police en cours au lieu de compter les caractères.

### Ajouté

- **Vente à crédit et gestion des dettes.** Une vente laissée impayée porte une
  échéance, proposée à partir d'un délai de règlement réglable, et exige de
  nommer son client : on ne relance pas « Client comptoir ». La facture affiche
  un bandeau « Vente à crédit » avec le reste dû et l'échéance.
- **Écran Créances** : total dû, échéances dépassées, part en retard, et
  classement par ancienneté en six tranches, du non échu au plus de quatre-vingt-dix
  jours. Vue par facture ou par client, encaissement direct, report d'échéance
  motivé et journalisé.
- **Lettre de relance imprimable**, qui reprend chaque facture, sa date, son
  échéance et son solde.
- Le tableau de bord distingue les impayés de ce qui est réellement en retard.

## [1.2.0] (2026-08-20)

### Corrigé

- **« Ouvrir le dossier » ne faisait rien.** L'application confiait une URL
  `file://` au navigateur, qui affichait une liste de fichiers dans un onglet au
  lieu d'ouvrir le Finder ou l'Explorateur, quand il ne l'ignorait pas
  simplement. Chaque système a sa commande, et c'est elle qui est appelée
  désormais.
- **Les textes se chevauchaient dans les rapports** dès que la fenêtre se
  resserrait. Trois défauts distincts : un libellé long venait toucher son
  montant faute d'écart imposé, les onglets se cassaient en deux lignes contre
  le sélecteur de période, et les graphiques placés en demi-carte étaient étirés
  horizontalement, ce qui déformait leurs étiquettes. Les lignes libellé et
  montant partagent maintenant une mise en forme commune, utilisée aussi sur les
  totaux de facture et de réception.

### Ajouté

- **Reprise d'un accès administrateur perdu.** Un mot de passe haché ne se
  retrouve pas : si le dernier administrateur oublie le sien, plus personne
  n'ouvre la boutique alors que les données restent lisibles sur le disque. Un
  fichier déposé dans le dossier de données déclenche au démarrage la
  réinitialisation, et le mot de passe provisoire est écrit à côté. La reprise
  est inscrite au journal d'audit, ne rejoue pas, et n'autorise rien d'autre que
  le choix d'un nouveau mot de passe. L'écran de connexion affiche la marche à
  suivre avec le chemin exact.
- **Effacement de toutes les données du poste**, depuis un nouvel onglet
  « Ce poste » des paramètres. L'opération demande de saisir le nom de
  l'entreprise, prend une dernière sauvegarde par défaut, et laisse le logiciel
  dans l'état d'un premier démarrage. Décocher la sauvegarde supprime aussi les
  archives et les exports, pour céder un poste sans rien y laisser.
- Le même onglet rassemble l'emplacement des données et la marche à suivre pour
  désinstaller sur chaque système.

- **Guide d'utilisation de 57 pages**, en PDF, de l'installation à la lecture
  des rapports. Ses captures sont prises sur l'application réelle, pilotée dans
  un navigateur sans affichage et branchée sur une boutique de démonstration
  produite par le vrai moteur : les chiffres qu'elles montrent sont cohérents
  entre eux, et rien n'y est reconstitué.
- Outils de production du guide : `tools/demo` construit la boutique de
  démonstration, `tools/captures` prend les captures et rend le PDF.
- La variable d'environnement `COMPTOIR_DATA_DIR` désigne le dossier de données
  à ouvrir. Elle sert à reprendre une sauvegarde, à tenir un second magasin sur
  la même machine, ou à travailler sur un jeu de démonstration sans toucher à
  celui du poste.

### Corrigé

- **L'espace entre un montant et son symbole était trop fine.** La typographie
  française distingue l'espace fine insécable, qui sépare les milliers, de
  l'espace insécable ordinaire, qui précède le symbole monétaire. Les deux
  étaient confondues, et « 228 920 FCFA » se lisait collé, surtout dans les
  grands chiffres du tableau de bord où le crénage est resserré.

## [1.1.0] (2026-08-18)

### Ajouté

- **Le logo de la boutique apparaît sur ses documents.** Il était téléversable
  depuis les paramètres, mais n'était affiché nulle part ailleurs. Il figure
  désormais dans le bandeau de marque des factures, devis, bons d'entrée et
  rapports, à l'échelle et sur fond blanc pour rester lisible. Un logo illisible
  ou abîmé est ignoré sans empêcher l'impression.
- **Le logo apparaît aussi dans l'application** : dans la barre latérale, à côté
  du nom de la boutique, et sur un bandeau en tête du tableau de bord avec la
  date du jour et l'encaissé. Le commerçant voit ainsi ce qui partira sur ses
  factures avant d'en imprimer une.
- **Bon de mouvement imprimable.** Une sortie de stock qui ne passe pas par une
  facture laissait une trace dans le journal, mais rien à faire signer. Chaque
  mouvement produit maintenant son justificatif, dont le titre suit l'opération :
  bon de sortie, bon de retour client, constat de défaut, mise au rebut, constat
  d'inventaire. Le document porte deux emplacements de signature, dont les
  libellés changent selon le sens du mouvement. Un vendeur peut l'imprimer, sans
  y voir la valorisation.

### Modifié

- **La fenêtre utilise la barre de titre du système.** Les boutons fermer,
  réduire et agrandir sont ceux de macOS, à la place où on les cherche. La barre
  dessinée par l'application se comportait toujours un peu différemment de
  toutes les autres fenêtres.

## [1.0.1] (2026-08-18)

### Corrigé

- **macOS annonçait que l'application était endommagée et refusait de
  l'ouvrir.** Le fichier `Info.plist` déclarait un exécutable nommé `comptoir`
  alors que le binaire s'appelle `Comptoir`. Le défaut restait invisible sur un
  Mac, dont le système de fichiers ignore la casse, mais `codesign` résout
  l'exécutable par cette clé : la signature du bundle était donc invalide, et
  macOS présente une signature invalide comme une corruption. Le clic droit sur
  « Ouvrir » ne suffisait même pas à contourner le blocage.

  L'application reste signée de façon ad hoc, faute de certificat Apple : macOS
  la signale comme provenant d'un développeur non identifié, ce qui se contourne
  au premier lancement. La marche à suivre est décrite dans le README.

### Ajouté

- Le workflow de publication vérifie la signature macOS avant de publier. Ce
  défaut ne se voyait pas sur le poste qui compile, seulement après
  téléchargement ; il est désormais rattrapé en amont.
- Signature Developer ID et certification Apple prises en charge par le workflow
  dès que le certificat est fourni en secret du dépôt.

## [1.0.0] (2026-08-18)

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
  première connexion, d'ici là, le compte n'ouvre rien.
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

[Non publié]: https://github.com/siskoai/stock/compare/v1.3.0...HEAD
[1.3.0]: https://github.com/siskoai/stock/releases/tag/v1.3.0
[1.2.0]: https://github.com/siskoai/stock/releases/tag/v1.2.0
[1.1.0]: https://github.com/siskoai/stock/releases/tag/v1.1.0
[1.0.1]: https://github.com/siskoai/stock/releases/tag/v1.0.1
[1.0.0]: https://github.com/siskoai/stock/releases/tag/v1.0.0
