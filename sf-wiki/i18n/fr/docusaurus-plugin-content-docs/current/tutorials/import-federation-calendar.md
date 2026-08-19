---
id: import-federation-calendar
title: Importer le calendrier fédéral
sidebar_position: 4
---

La FFForce publie chaque saison sous forme de calendrier annuel au format PDF.
Si vous êtes coach, vous pouvez envoyer ce fichier et StrongFish en extraira les
compétitions — dates, noms, tout — au lieu de vous faire ressaisir quarante
lignes.

## Où trouver le fichier

La fédération le met en ligne sur son propre site, à la page
[Compétitions Force Athlétique National](https://www.ffforce.fr/fr/force-athletique-ffforce/national-force-athletique/competitions-force-athletique-national/saison-2026-national-fa.html).
Téléchargez le PDF de la saison depuis cette page — le planning de l'année
entière, avec les mois en colonnes, et non le formulaire d'une compétition
isolée.

**Qui peut le faire :** un coach, pour son club, et un superadmin, qui peut
aussi placer une saison sur le calendrier public. C'est la même permission que
pour ajouter un événement à la main, puisqu'un import n'est rien d'autre que
cela, en plus grand nombre.

## L'envoyer

1. Ouvrez **Événements**.
2. Cliquez sur **Importer un calendrier**.
3. Choisissez le club auquel la saison appartient. Un superadmin peut laisser ce
   champ vide pour publier sur le calendrier public.
4. Sélectionnez le PDF.

La lecture prend une seconde ou deux, et les compétitions apparaissent aussitôt
sur votre calendrier.

## Ce qui est repris

**Chaque compétition devient un événement sur la journée entière.** Le planning
note des jours, jamais une heure : un championnat national est inscrit dans la
grille sous la forme « 13-15/05 », et c'est ce que cela veut dire. Rien
n'invente un départ à 9 h.

**Les couleurs suivent.** Sur la page imprimée, la catégorie — fédérale,
européenne, mondiale, meeting spécial — n'est indiquée que par la couleur dans
laquelle la compétition est tramée. StrongFish conserve cette couleur sur
l'événement, pour qu'un mois de dates importées reste aussi lisible à l'écran
qu'il l'était sur le papier.

**Les dates viennent d'abord de ce qui est écrit.** Quand une compétition porte
ses dates dans son libellé, ce sont celles-là qui sont retenues, sous quelque
forme qu'elles aient été saisies :

| Sur le calendrier | Lu comme |
| --- | --- |
| `13/05 - 15/05` | du 13 au 15 mai |
| `13-05 / 15-05` | du 13 au 15 mai |
| `13-15/05` | du 13 au 15 mai |
| `24 au 27/07` | du 24 au 27 juillet |
| `30 au 1er Nov` | du 30 octobre au 1er novembre |
| `9-15` | du 9 au 15 du mois de la colonne |

Quand une entrée ne porte aucune date, c'est la bande de couleur à côté d'elle
qui sert : les jours qu'elle couvre dans sa propre colonne.

## Réimporter un calendrier révisé

La fédération révise la saison en cours d'année et la republie. Envoyez le
nouveau fichier de la même façon : ce qui figure déjà sur votre calendrier est
laissé tel quel, et seules les nouveautés sont ajoutées. Le résultat vous
indique combien ont été ajoutées et combien étaient déjà là.

## Les entrées non datées

Un planning est un document humain. Quelques entrées ne portent ni date ni
trame — une note en marge, une échéance écrite en travers. Elles sont listées
par leur nom à la fin de l'import, pour que vous puissiez les ajouter à la main
plutôt que de découvrir le trou en mars.

Si le fichier entier est refusé, vérifiez que vous avez bien téléchargé le
planning annuel et non l'image scannée d'un calendrier : l'import lit le texte
et les formes du PDF, et une photographie de calendrier ne contient rien à lire.
