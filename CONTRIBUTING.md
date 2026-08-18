# Contribuer à Comptoir

Merci de l'intérêt porté au projet. Ce document dit comment proposer un
changement, et quelles attentes s'appliquent au code.

## Avant de commencer

Lisez la [licence](LICENSE). Elle autorise l'usage, la modification et la
redistribution, y compris commerciale, en contrepartie du maintien de la
paternité de l'éditeur (article 3). Une contribution qui retirerait ou
affaiblirait la marque SISKO, la mention de paternité ou la vérification
d'intégrité du paquet `internal/brand` ne sera pas acceptée, quelle que soit sa
justification technique.

En proposant une contribution, vous acceptez qu'elle soit distribuée sous cette
même licence.

## Ouvrir une issue

* **Un bug** : utilisez le modèle prévu. Indiquez la version, le système, ce que
  vous attendiez et ce qui s'est produit. Un chiffre faux est plus utile avec
  les montants exacts qui l'ont produit.
* **Une idée** : décrivez d'abord le problème que vous rencontrez, avant la
  solution que vous imaginez. La plupart des bonnes fonctionnalités viennent
  d'une gêne bien décrite.
* **Une faille de sécurité** : surtout pas d'issue publique. Voir
  [SECURITY.md](SECURITY.md).

## Préparer son poste

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails doctor          # signale les dépendances système manquantes
make dev              # lance l'application avec rechargement à chaud
```

## Ce qui est attendu d'un changement

```sh
make check            # tests, vet, formatage, typage du frontend
```

Cette commande doit passer avant toute proposition. L'intégration continue la
rejoue sur les trois systèmes.

### Règles qui ne se négocient pas

Elles tiennent le logiciel debout. Un changement qui les enfreint sera refusé
même s'il fonctionne.

1. **Aucun flottant sur l'argent.** Les montants sont des entiers en centièmes
   d'unité monétaire. Les divisions s'arrondissent au plus proche, jamais par
   troncature, et le reliquat se reporte sur la dernière ligne.
2. **Aucune quantité de stock ne change sans mouvement.** Si votre code modifie
   `Product.Quantity`, il doit écrire un mouvement correspondant, ou passer par
   `applyStock`.
3. **Le contrôle d'accès vit dans les services.** Masquer un bouton n'est pas
   une protection. Une donnée qu'un rôle ne doit pas voir ne doit pas lui être
   envoyée.
4. **Tout ce qui s'écrit se journalise.** Une opération sensible ajoute une
   entrée au journal d'audit.
5. **Les messages d'erreur s'adressent à un commerçant**, en français, et disent
   quoi faire. « stock insuffisant pour Clavier : 3 disponibles, 5 demandés »
   plutôt que « validation failed ».

### Style

* Le code, les commentaires et les messages sont en français.
* Un commentaire explique **pourquoi**, pas **quoi**. Le code dit déjà ce qu'il
  fait.
* `gofmt` fait autorité sur le formatage Go.
* Pas de bibliothèque de composants ni de graphes côté interface. Le rendu doit
  être identique sur les trois systèmes, et le poids du bundle compte.
* Une dépendance nouvelle se justifie dans la proposition. Le projet en compte
  très peu, volontairement.

### Tests

Un correctif de bug s'accompagne du test qui échouait avant. Une fonctionnalité
s'accompagne des tests de ses règles, y compris des refus attendus.

Les tests sont écrits en français, et leurs messages d'échec disent la valeur
obtenue, la valeur attendue, et pourquoi cela compte.

## Proposer le changement

1. Créez une branche à partir de `main`.
2. Un commit par idée, avec un message qui explique la raison du changement et
   pas seulement son contenu.
3. Ouvrez une pull request en décrivant ce que vous avez vérifié à la main, en
   plus des tests automatiques.

## Ce qui ne sera probablement pas accepté

* Une refonte visuelle sans problème d'usage identifié.
* L'ajout d'un appel réseau, quel qu'il soit. Le logiciel ne se connecte à rien,
  c'est une promesse faite aux utilisateurs et inscrite à l'article 2 de la
  licence.
* De la télémétrie, des statistiques d'usage, une vérification de licence en
  ligne.
* Le retrait ou l'affaiblissement de la marque de l'éditeur.
