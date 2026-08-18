# Règles de calcul

Ce document décrit comment Comptoir calcule ce qu'il affiche, et surtout ce
qu'il ne calcule pas. Il s'adresse à qui doit vérifier un chiffre — vous, votre
comptable, ou un développeur qui reprend le code.

---

## Les montants sont des entiers

Tous les montants sont stockés en **centièmes d'unité monétaire**, dans des
entiers 64 bits. 1 500 FCFA valent `150000`. Un prix de 12,50 € vaut `1250`.

Les nombres à virgule flottante sont écartés : `0,1 + 0,2` ne vaut pas `0,3` en
binaire, et sur mille lignes de facture cet écart devient visible. Un entier
est exact.

Quand une division est nécessaire — un pourcentage, une répartition au prorata
— le résultat est **arrondi au plus proche**, jamais tronqué. Les reliquats
d'arrondi sont reportés sur la dernière ligne, de sorte que la somme des lignes
égale toujours le total imprimé, au centième près.

Le franc CFA n'ayant pas de subdivision en usage, le réglage par défaut affiche
zéro décimale : les centièmes existent en interne mais n'apparaissent nulle
part.

---

## Taxes

Le taux par défaut s'applique à toute ligne dont le taux n'est pas précisé. Une
ligne peut porter **un taux explicite de 0 %** : une exonération doit pouvoir
s'exprimer, et elle se distingue d'un taux non renseigné.

**Prix saisis hors taxe** (réglage par défaut) : la taxe s'ajoute au prix.
**Prix saisis TTC** : la base hors taxe est extraite du prix affiché, par
division. Le total TTC retrouvé est alors exactement le prix affiché.

La taxe porte sur le prix **effectivement payé** : les remises de ligne et la
remise globale réduisent la base taxable avant application du taux. La remise
globale est répartie sur les lignes au prorata de leur montant hors taxe.

---

## Coût moyen pondéré (CUMP)

Le coût d'un article n'est pas le prix de son dernier achat, mais la moyenne
pondérée de tout ce qui est en stock :

```
nouveau coût = (ancienne quantité × ancien coût + quantité reçue × coût de revient)
               ÷ (ancienne quantité + quantité reçue)
```

Le **coût de revient** d'une réception comprend le prix d'achat net de remise,
plus la quote-part des frais annexes — transport, douane, manutention —
répartie au prorata de la valeur des lignes. Négliger ces frais surestime les
bénéfices, parfois de plusieurs points de marge.

Un stock nul ou négatif est ignoré : le nouveau coût est celui de l'entrée.

Le recalcul a lieu **à l'intérieur de la transaction de stock**, ligne après
ligne, sur les quantités à jour. Un même article présent sur deux lignes d'un
même bon d'entrée est donc correctement cumulé.

---

## Marge

```
marge brute = chiffre d'affaires net HT − coût des marchandises vendues
```

Le coût des marchandises vendues est figé **au moment de la sortie de stock**,
au coût moyen de ce jour-là. Une facture émise ne voit plus sa marge bouger,
même si le coût de l'article change ensuite : c'est ce qui rend les rapports
d'une période close stables.

Cas particulier du devis : les prix de vente sont ceux du devis — c'est un
engagement pris envers le client — mais les coûts sont réalignés sur le coût
moyen du jour de l'émission. La marge reflète ce que la marchandise a
réellement coûté au moment où elle sort.

---

## Compte de résultat

```
chiffre d'affaires net HT   ventes hors taxes, remises déduites
− coût des marchandises     coût moyen des articles sortis
= marge brute

− charges d'exploitation    loyer, salaires, électricité, transport…
− pertes sur rebuts         valeur des articles mis au rebut
= résultat d'exploitation
```

Seules les **factures émises** comptent. Les devis n'ont aucun effet ; les
factures annulées sont exclues du chiffre d'affaires mais conservées, pour que
la numérotation reste continue.

Les taxes facturées ne sont pas un produit : elles sont collectées pour l'État
et présentées séparément, comme un montant à reverser.

---

## Trésorerie estimée

```
encaissements clients − achats fournisseurs − charges = flux estimé
```

Ce chiffre repose sur une hypothèse : **les achats fournisseurs sont réglés à
la réception**. C'est la situation courante en commerce de détail, mais
Comptoir ne suit pas les échéances fournisseurs et ne peut donc pas le vérifier.
Le flux affiché situe une tendance ; il n'arrête pas une trésorerie.

---

## Situation patrimoniale

L'écran « Situation » présente ce que Comptoir suit réellement :

- la valeur du stock vendable et du stock défectueux, au coût moyen ;
- les créances clients — les factures dont le solde est positif ;
- le résultat cumulé depuis l'ouverture ;
- les taxes facturées restant à reverser.

**Ce n'est pas un bilan.** Comptoir ne tient ni comptes bancaires, ni caisse, ni
immobilisations, ni dettes fournisseurs, ni capital. Les rubriques absentes le
sont volontairement : mieux vaut un état partiel et honnête qu'un bilan
d'apparence complète mais faux.

---

## Limites connues

**Annulation d'un bon d'entrée.** Le stock est retiré, mais le coût moyen n'est
pas recalculé à rebours : il refléterait un historique d'achats qui n'existe
plus, et le recalcul supposerait de rejouer toutes les entrées depuis l'origine.
Le coût se réaligne naturellement à la réception suivante. L'annulation est
refusée si une partie de la marchandise a déjà été vendue.

**Annulation d'une facture réglée.** La marchandise revient en stock et
l'acompte encaissé est signalé comme montant à rembourser. Le remboursement
lui-même n'est pas un mouvement suivi : il se constate hors du logiciel.

**Retours clients.** Le mouvement ne touche que le stock physique. L'avoir
financier se fait par annulation de la facture d'origine — Comptoir n'émet pas
de note de crédit distincte.

**Pas de valorisation FIFO ou LIFO.** Le coût moyen pondéré est le seul mode
retenu. Il convient au commerce de détail ; il ne conviendrait pas à des lots
dont le coût varie fortement et qu'il faut suivre individuellement.
