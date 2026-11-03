---
id: velocity-based-rpe
title: L'effort basé sur la vitesse
sidebar_position: 8
---

Comment la vitesse d'une barre devient un chiffre pour la difficulté d'une série - la science derrière un « RPE mesuré », expliquée indépendamment de tout traqueur ou marque.

## Le problème de demander ce qu'on a ressenti

Le RPE - l'effort perçu - est la façon dont la plupart des pratiquants notent l'effort : une échelle de 1 à 10 où 10 est un effort maximal, rien en réserve, et où les chiffres plus bas sont de plus en plus faciles. Son cousin le RIR, les répétitions en réserve, est la même information dans l'autre sens :

```
RIR = 10 - RPE
```

Un RPE de 8 veut dire environ deux répétitions en réserve. L'ennui, c'est que le RPE est un jugement, et le jugement dérive : les débutants sous-estiment les séries dures, le même effort reçoit un chiffre différent un bon et un mauvais jour, et ce qu'on note après coup est teinté par la façon dont la série s'est terminée plutôt que dont elle s'est déroulée.

La vitesse de la barre ne dérive pas. Elle se mesure, et elle porte l'essentiel de la même information.

## Premier fait : la relation charge-vitesse

Pour un pratiquant donné sur un exercice donné, la vitesse à laquelle la barre monte sur la phase concentrique - la moitié montante de la répétition - est étroitement liée à la charge. Les charges légères montent vite, les lourdes lentement, et la relation est quasi linéaire sur la plus grande partie de la plage :

```
v = a - b * charge
```

où `v` est la vitesse concentrique moyenne de la répétition, `charge` est un pourcentage du maximum sur une répétition, et `a` et `b` sont des constantes ajustées à ce pratiquant sur cet exercice. Tracez quelques répétitions à des charges connues et les points tombent sur une droite - un profil charge-vitesse. Parce qu'il est très reproductible, la vitesse d'une seule répétition rapide suffit à relire la charge sur la droite à quelques pour cent près.

La vitesse concentrique moyenne - la vitesse moyenne sur toute la plage montante - est la mesure habituelle plutôt que la vitesse de pic, parce qu'elle est plus stable d'une répétition à l'autre.

## Deuxième fait : l'échec survient à une vitesse fixe

La dernière répétition qu'un pratiquant peut arracher, celle juste avant l'échec, monte à peu près à la même vitesse quelle que soit la charge. Une charge bonne pour deux répétitions rampe sur la deuxième ; une charge bonne pour dix rampe sur la dixième, à ce même rythme. Cette vitesse est le **seuil de vitesse minimale** (MVT), et il est à peu près constant pour un exercice et un pratiquant donnés :

```
MVT ~= vitesse de la dernière répétition possible   (propre à l'exercice et au pratiquant)
```

Pour un squat, il tourne autour de `0,30 m/s` chez beaucoup de pratiquants, plus bas au développé couché, différent encore au soulevé de terre. La valeur exacte importe moins que le fait qu'elle soit stable.

## En les combinant : les répétitions en réserve d'après la vitesse

Si l'échec est une vitesse connue et que la vitesse de chaque répétition est mesurée, alors l'écart d'une répétition au-dessus du MVT dit ce qu'il reste - une répétition bien plus rapide que le MVT a des répétitions d'avance, une répétition qui s'en approche est proche de l'échec. Cette marge est la matière première d'une estimation du RIR, et de là du RPE :

```
RIR ~= (v_actuelle - v_MVT) / k        RPE = 10 - RIR
```

où `k` est la vitesse perdue par répétition à mesure que la série fatigue, ajustée par exercice. Plus les répétitions d'un pratiquant chutent vite, plus `k` est grand, et moins une marge de vitesse donnée achète de répétitions.

## Troisième signal : la perte de vitesse sur la série

Il existe une lecture qui n'a besoin ni d'un maximum sur une répétition ni d'un profil : de combien la barre a ralenti entre la répétition la plus rapide de la série et l'actuelle.

```
perte de vitesse = (v_max - v_actuelle) / v_max
```

La fatigue dans une série est ordonnée - un pourcentage de chute donné arrive à une proximité de l'échec donnée pour un exercice donné. Une perte de 20 % au squat laisse la plupart des pratiquants à plusieurs répétitions de l'échec ; poussée à 40 %, ils en sont proches. Les programmes autorégulés plafonnent souvent une série sur une perte de vitesse plutôt que sur un nombre de répétitions, précisément pour cela : la série s'arrête à un effort constant plutôt qu'à un nombre constant.

## Un exemple chiffré

Prenons une série de squat, et supposons que le profil et l'historique du pratiquant donnent :

- MVT ~= `0,30 m/s`
- vitesse perdue par répétition en haut de plage ~= `0,05 m/s`, donc `k = 0,05`

La série est réalisée et le capteur rapporte sa dernière répétition à `0,45 m/s`. Alors :

```
RIR ~= (0,45 - 0,30) / 0,05 = 3 répétitions en réserve
RPE ~= 10 - 3 = 7
```

Si la dernière répétition était plutôt montée à `0,35 m/s` :

```
RIR ~= (0,35 - 0,30) / 0,05 = 1
RPE ~= 9
```

Le même calcul, mené sur des vitesses mesurées plutôt que sur une sensation, est ce qui transforme une série suivie en un chiffre.

## Ce qu'exige la précision

- **C'est personnel.** `a`, `b`, le MVT et `k` sont propres au pratiquant, lus sur ses propres séries au fil du temps, pas pris dans une table. Une estimation non calibrée est une supposition.
- **C'est par exercice.** Squat, développé couché et soulevé de terre ont chacun leur profil et leur MVT ; un modèle de l'un ne se transfère pas à un autre.
- **La technique doit être honnête.** La mesure suppose que la concentrique est faite avec intention - des répétitions volontairement lentes, partielles ou lancées par un rebond se lisent comme la mauvaise vitesse.
- **Arrachés et lancés diffèrent.** Là où la barre accélère puis décélère près du haut, la vitesse moyenne sur toute la plage est plus honnête que le pic.

## Comment StrongFish s'en sert

StrongFish ne calcule rien de tout cela lui-même. Un traqueur de vitesse de barre mesure la série, en tire les vitesses par répétition et le RPE mesuré, et StrongFish lit ce résultat fini et le trace à côté du RPE que le pratiquant a annoncé, une barre par répétition. La méthode ci-dessus est ce que le chiffre veut dire ; StrongFish ne fait jamais qu'afficher ce que le traqueur a conclu.
