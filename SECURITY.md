# Politique de sécurité

## Versions suivies

| Version | Correctifs de sécurité |
|---------|------------------------|
| 1.0.x   | oui                    |
| < 1.0   | non                    |

Seule la dernière version publiée reçoit des correctifs. Comptoir étant une
application locale sans mise à jour automatique, la mise à jour se fait en
téléchargeant la version publiée la plus récente.

## Signaler une faille

Une faille dans un logiciel qui tient la caisse d'un commerce mérite d'être
signalée en privé plutôt que publiée.

**N'ouvrez pas d'issue publique** pour un problème de sécurité.

Utilisez plutôt l'onglet **Security > Report a vulnerability** du dépôt, qui
ouvre un signalement privé visible des seuls mainteneurs.

Indiquez, autant que possible :

* la version de Comptoir et le système d'exploitation,
* les étapes qui reproduisent le problème,
* ce qu'un attaquant obtiendrait,
* si vous avez déjà un correctif en tête.

Vous recevrez un accusé de réception sous 7 jours. Si le problème est confirmé,
un correctif est publié avant toute divulgation, et votre nom figure dans le
journal des versions si vous le souhaitez.

## Ce que le logiciel protège

Comptoir est une application de poste, sans réseau. Son modèle de menace est
décrit en détail dans [docs/SECURITE.md](docs/SECURITE.md). En résumé, il
protège :

* l'accès aux données par une personne non autorisée assise devant le poste,
* le dépassement de rôle, un vendeur n'obtenant ni prix d'achat, ni marge, ni
  charges, le filtrage étant appliqué côté Go et non à l'écran,
* la traçabilité, par un journal d'audit en ajout seul,
* les mots de passe, hachés avec bcrypt,
* l'intégrité des données, par écriture atomique et transactions,
* les archives de sauvegarde, contre les entrées piégées à la restauration.

## Ce qu'il ne protège pas, et qui n'est donc pas une faille

Les points suivants sont des limites assumées, documentées, et ne constituent
pas des vulnérabilités :

* **Les fichiers de données sont lisibles.** C'est un choix : il rend vos
  données récupérables sans le logiciel. Le chiffrement du disque relève du
  système d'exploitation (BitLocker, FileVault, LUKS).
* **Un poste compromis reste compromis.** Un logiciel malveillant déjà installé
  peut lire la mémoire et les fichiers. Aucune application locale ne s'en
  défend seule.
* **Quiconque détient le code source peut le modifier.** La vérification
  d'intégrité de la marque n'a pas la prétention de l'empêcher, seulement de
  rendre une altération délibérée et traçable. Voir l'article 3.4 de la
  [licence](LICENSE).

## Portée

Sont dans le périmètre : le code de ce dépôt, les exécutables publiés dans les
versions, et les fichiers de données qu'ils produisent.

Sont hors périmètre : les dépendances tierces (signalez-les à leurs auteurs),
les versions modifiées par des tiers, et les configurations où le poste lui-même
est déjà compromis.
