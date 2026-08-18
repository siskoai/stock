# Modèle de menace

Comptoir est une application locale, sans réseau. Ce document dit ce qu'elle
protège, ce qu'elle ne protège pas, et pourquoi.

---

## Ce que le logiciel protège

**Un accès non autorisé au poste laissé sans surveillance.** Une session est
nécessaire pour lire ou écrire quoi que ce soit, et elle se ferme seule après
un délai d'inactivité réglable (une heure par défaut).

**Un vendeur qui dépasserait son rôle.** Les prix d'achat, les marges, les
charges, la valorisation du stock, les comptes et les paramètres lui sont
inaccessibles. Ce filtrage est appliqué **côté Go**, dans les services : les
projections envoyées à l'interface ne contiennent tout simplement pas les
champs interdits. Masquer une colonne n'est pas une protection ; ne pas envoyer
la donnée en est une.

**La traçabilité.** Chaque action sensible — création, modification,
suppression, annulation, connexion, réinitialisation de mot de passe — est
inscrite dans un journal d'audit en ajout seul, avec l'auteur et l'horodatage.
Aucune fonction du logiciel ne permet de l'effacer ni de le modifier.

**Les mots de passe.** Ils sont hachés avec bcrypt à coût 12, soit environ
250 ms par vérification sur une machine de bureau : assez lent pour rendre une
attaque par dictionnaire hors ligne coûteuse, assez rapide pour que la
connexion reste instantanée à l'usage. Cinq échecs consécutifs bloquent le
compte pendant une minute. Une tentative sur un identifiant inexistant déclenche
une comparaison factice, pour que le temps de réponse ne révèle pas l'existence
du compte.

**Les archives de sauvegarde.** À la restauration, toute entrée qui tenterait
de s'écrire hors du dossier de destination est refusée (« Zip Slip »), la
taille extraite est plafonnée (« zip bomb »), et une archive qui ne ressemble
pas à une sauvegarde Comptoir est rejetée. Une sauvegarde de sécurité est
systématiquement prise avant toute restauration.

---

## Ce que le logiciel ne protège pas

**Un accès administrateur au système de fichiers.** Les fichiers de données sont
du JSON lisible, par choix : cela rend l'inspection et la récupération manuelles
possibles, et c'est ce qui garantit que vos données vous survivent au logiciel.
Quelqu'un qui peut lire ces fichiers lit tout, sans passer par Comptoir.

Le chiffrement du disque est la bonne couche pour s'en protéger, et c'est le
système d'exploitation qui la fournit :

- **Windows** — BitLocker (Paramètres → Confidentialité et sécurité →
  Chiffrement de l'appareil) ;
- **macOS** — FileVault (Réglages → Confidentialité et sécurité) ;
- **Linux** — LUKS, à configurer à l'installation.

**Un poste compromis.** Un logiciel malveillant installé sur la machine peut
lire la mémoire, enregistrer les frappes ou modifier les fichiers. Aucune
application locale ne peut s'en défendre seule.

**La perte du matériel.** Sauvegardez régulièrement sur un support externe.
Une archive qui vit sur le même disque que les données ne protège pas d'une
panne de ce disque.

---

## Rôles et permissions

| Domaine | Administrateur | Gérant | Vendeur |
|---|:---:|:---:|:---:|
| Consulter le catalogue et le stock | ● | ● | ● |
| Enregistrer des ventes | ● | ● | ● |
| Créer un client au moment de la vente | ● | ● | ● |
| Modifier les articles et les catégories | ● | ● | |
| Mouvements de stock, inventaire, retours | ● | ● | |
| Achats et réceptions fournisseur | ● | ● | |
| Charges d'exploitation | ● | ● | |
| Rapports financiers, marges, coûts | ● | ● | |
| Sauvegardes et restauration | ● | ● | |
| Suppressions définitives | ● | ● | |
| Comptes et journal d'audit | ● | | |
| Paramètres de l'application | ● | | |

Deux garde-fous empêchent de se verrouiller dehors : on ne peut ni désactiver
son propre compte, ni retirer son rôle au dernier administrateur actif.

Un mot de passe réinitialisé par un administrateur n'ouvre rien tant qu'il n'a
pas été remplacé : le titulaire peut ouvrir une session, mais toute autre action
est refusée jusqu'au changement. Un mot de passe provisoire transmis de vive
voix ne doit pas rester une clé utilisable indéfiniment.

---

## Intégrité des données

**Écriture atomique.** Chaque enregistrement passe par un fichier temporaire,
une synchronisation disque, puis un renommage — opération atomique sur NTFS
comme sur APFS et ext4. Une coupure de courant en pleine écriture laisse
l'ancien fichier intact, jamais un fichier à moitié écrit.

**Transactions.** Une opération qui touche plusieurs enregistrements — émission
de facture, réception, inventaire — est appliquée en une seule écriture par
collection : soit tout passe, soit rien ne change. Si le journal des mouvements
ne peut pas être écrit, les quantités sont remises exactement dans leur état
antérieur.

**Fichier illisible.** Un fichier de données que le logiciel ne parvient pas à
relire est mis de côté sous l'extension `.corrupt` plutôt qu'écrasé, pour
permettre une récupération manuelle.

**Numérotation.** Les numéros de facture et de bon d'entrée sont réservés avant
écriture et rendus si le document n'est finalement pas créé : ni doublon, ni
trou. Les compteurs ne sont pas modifiables depuis l'interface — les abaisser
redistribuerait des numéros déjà attribués.

**Schéma.** Des données créées par une version plus récente du logiciel sont
refusées à l'ouverture, avec un message explicite, plutôt que lues partiellement.
Une sauvegarde est prise avant toute migration de format.

---

## Signaler un problème

Une faille dans un logiciel qui tient la caisse d'un commerce mérite d'être
signalée en privé plutôt que publiée. Ouvrez une issue sans détail exploitable
en demandant un contact, ou écrivez directement au responsable du dépôt.
