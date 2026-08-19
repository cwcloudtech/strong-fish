---
id: vocabulary
title: RPE, 1RM et e1RM
sidebar_position: 3
---

Trois nombres traversent toute l'application. Ils méritent dix minutes, car
toutes les charges affichées par StrongFish en découlent.

## 1RM — votre maximum sur une répétition

La charge la plus lourde que vous pouvez soulever une fois sur un mouvement
donné. C'est le seul nombre que vous saisissez vous-même, et tout le reste en
découle.

Vous n'avez pas besoin de l'avoir testé récemment, ni même du tout : une
estimation honnête suffit pour démarrer. L'application est construite en
supposant qu'il changera - mettez-le à jour et tous vos programmes se
recalculent, puisqu'aucune charge n'est stockée, seulement calculée.

## RPE — à quel point une série était difficile

**Rate of Perceived Exertion**, sur une échelle de 1 à 10. En powerlifting, elle
est utilisée dans un sens précis : *combien de répétitions auriez-vous pu faire
en plus ?*

| RPE | Signification | RIR |
| --- | --- | --- |
| 10 | Aucune répétition en réserve. Un vrai maximum. | 0 |
| 9,5 | Peut-être une répétition, peut-être pas. | 0–1 |
| 9 | Une répétition en réserve. | 1 |
| 8,5 | Une certaine, peut-être deux. | 1–2 |
| **8** | **Deux répétitions en réserve.** | **2** |
| 7,5 | Deux certaines, peut-être trois. | 2–3 |
| 7 | Trois répétitions en réserve. | 3 |
| 6 | Quatre répétitions en réserve. | 4 |

Donc **RPE 8 = RIR 2** : vous vous arrêtez avec deux répétitions en réserve.
« 5 répétitions @ RPE 8 » signifie *prenez une charge avec laquelle vous auriez
pu en faire 7, et faites-en 5*.

### Pourquoi les coachs programment à la RPE

Un pourcentage est une promesse sur une journée que vous n'avez pas encore
vécue. La RPE est une consigne sur la journée que vous vivez réellement : si
vous avez mal dormi, « 3 @ RPE 8 » est une barre plus légère que la semaine
dernière, et c'est toujours le bon entraînement.

### Comment StrongFish convertit la RPE en kilos

À partir de la **table RTS/Tuchscherer** : un tableau donnant la fraction de
votre maximum que représente un nombre de répétitions à une RPE donnée. Un
single à RPE 10 vaut 100 % par définition ; 5 répétitions à RPE 8 valent environ
76 %.

La table, pas une formule. L'importateur de StrongFish a d'abord été écrit avec
l'équation d'Epley et produisait des charges en désaccord avec le tableur du
coach sur 12 séries sur 15 : c'est à partir de la table que les vrais programmes
sont écrits, donc c'est à partir d'elle que l'application calcule.

:::note Les séries sans RPE
Un coach peut prescrire un pourcentage de votre 1RM à la place. Ceux-là sont
utilisés tels quels : c'est un choix délibéré, pas un oubli.
:::

## e1RM — ce qu'une série dit de votre maximum

**Estimated 1RM** : la table lue à l'envers. Si vous avez fait 5 répétitions à
100 kg en les jugeant à RPE 8, cela représente 76 % de quelque chose : votre
maximum ce jour-là était donc d'environ 132 kg.

C'est ce qui rend le retour à la RPE utile à noter. Chaque série enregistrée
devient une estimation de votre maximum du jour, sans jamais avoir à en tester
un, et un e1RM qui monte au fil d'un bloc est la preuve que le bloc fonctionne.

Par construction, une série exécutée exactement comme prescrite produit un e1RM
égal au 1RM à partir duquel elle a été calculée. C'est la cohérence qui manquait
au tableur d'origine, et pour laquelle l'application a été construite.

## Mouvements de compétition

Le squat, le développé couché et le soulevé de terre sont marqués comme
mouvements de compétition. Une variante - un développé Larsen, un squat tempo,
un pin bench - est programmée à partir du maximum du mouvement de compétition
plutôt que du sien : vous n'avez pas à tester un maximum sur chaque accessoire
que vous avez pratiqué.
