---
id: ai-assistant
title: Assistant IA
sidebar_position: 7
---

StrongFish peut soumettre votre entraînement à un modèle de langage : un chat qui répond comme un coach de powerlifting, une analyse d'une séance que vous venez de terminer, et une analyse des quatre-vingt-dix derniers jours de vos données fitness.

L'application ne fournit pas de modèle. Vous connectez **votre propre compte** chez un fournisseur - la clé est la vôtre, la facture aussi - et l'application la dépense pour vous. Vous pouvez aussi prêter votre connexion à d'autres membres, comme vous prêtez un stockage vidéo.

## Connecter un modèle

Dans **Paramètres**, sous **Connexions IA**, ajoutez-en une :

| Champ | Ce que c'est |
| --- | --- |
| **Nom** | Comme vous l'appelez. Laissé vide, il reprend le modèle - `claude-sonnet-5-5-a9bffea3` - ce que le champ propose d'ailleurs pendant que vous tapez. |
| **Fournisseur** | OpenAI, Mistral, Anthropic (Claude), Google Gemini, OpenRouter ou DeepSeek. C'est le format d'échange, pas un engagement envers la marque. |
| **Adresse** | Où partent les requêtes. Elle est pré-remplie selon le fournisseur choisi et reste **modifiable** : visez un LiteLLM, un Ollama, un compte OpenRouter, tout ce qui parle le même protocole. |
| **Modèle** | Quel modèle interroger. Le champ suggère ceux que votre fournisseur est connu pour servir et **accepte n'importe quel nom saisi** : un modèle sorti hier fonctionne dès sa sortie. |
| **Clé d'API** | La clé de votre fournisseur. Elle reste sur le serveur et n'est jamais renvoyée au navigateur ni au téléphone : une fois enregistrée, le champ propose de la remplacer plutôt que de l'afficher. |

Chaque texte d'exemple suit le fournisseur choisi : ce que les champs
affichent, ce sont l'adresse, le modèle et le format de clé de ce
fournisseur-là.

Une connexion suffit. **Si vous en avez plusieurs** - la vôtre et celle que
votre coach vous a prêtée - on vous demande laquelle dépenser : un sélecteur
au-dessus du chat, et un menu sur chaque bouton IA. L'application ne devine
jamais quand il y a un choix, et ne demande jamais quand il n'y en a pas.

### Laquelle répond par défaut

Vos connexions forment une liste, et **la première répond** quand vous ne
choisissez pas - c'est aussi la première ligne du menu de chaque bouton IA.
Les flèches à côté de chacune la déplacent ; une connexion qu'on vous a
partagée se range à la fin et reste où son propriétaire l'a mise.

Une connexion que vous venez d'ajouter va en bas de liste : coller une clé ne
change donc jamais en douce la souscription sur laquelle partent vos
questions.

### La prêter à d'autres membres

L'icône de groupe à côté d'une connexion ouvre sa liste d'accès. Les personnes que vous y ajoutez peuvent **poser des questions avec, et rien d'autre** : elles ne voient jamais la clé, ne peuvent pas modifier la connexion ni la repartager. Révoquer prend effet immédiatement.

C'est ainsi qu'un coach paie une seule clé et en fait profiter tous ses athlètes.

## Le chat coach

**Coach IA** dans le menu. Posez vos questions sur votre pratique : programmation, technique, récupération, jour de compétition. Le modèle répond dans la langue configurée sur votre compte.

Il sait déjà qui vous êtes. Chaque question emporte **votre fiche** : âge,
poids de corps, taille, tous vos 1RM enregistrés, le bloc que vous suivez avec
tout ce que vous y avez consigné, et ce que votre montre a compté ces
vingt-quatre dernières heures. « Je squat ce soir ? » est donc répondu au
regard de la séance de ce soir et des pas d'aujourd'hui, pas en général - et
les charges évoquées sont les kilos réellement prescrits par votre programme.

La fiche est reconstruite à chaque question plutôt qu'inscrite dans la
conversation : un vieux fil ne répond donc jamais avec un maximum que vous
avez depuis dépassé.

### Quelle IA répond

Si vous avez plusieurs souscriptions, **le bouton d'envoi demande laquelle** :
il les ouvre par leur nom, comme les boutons IA d'une séance. Avec une seule
souscription, il envoie simplement.

Le choix se fait par message, pas par conversation : un même fil peut tourner
sur votre clé pour une question et sur celle de votre coach pour la suivante.
Chaque souscription pointe vers son propre modèle : choisir un nom choisit
donc le modèle avec.

C'est pourquoi **chaque réponse indique ce qui l'a produite et quand**, en
petites italiques en dessous : *Généré par claude-opus-5-5 le 2 novembre
2026*. La conversation exportée porte la même ligne sous chaque réponse.

Chaque échange est conservé : la liste de gauche est votre historique, pas celui de cet onglet. Pour chaque conversation vous pouvez :

- la **renommer** - le titre reprend d'abord votre première question ;
- **partager un lien** - toute personne qui l'a peut lire le fil, comme une conversation ChatGPT partagée. Recliquez sur le bouton de lien pour le révoquer : il cesse aussitôt de fonctionner ;
- l'**exporter** en PDF ou en image, pour la garder ou l'envoyer ;
- la **supprimer** - une par une, plusieurs d'un coup avec les cases à cocher, ou tout l'historique.

## L'analyse d'une séance

Ouvrez un programme assigné, ouvrez une séance : le bouton **Analyse de l'IA** se trouve sous ses séries.

Il est **inactif tant que la séance n'est pas terminée** - tous les sets enregistrés. Une analyse de trois séries sur huit est l'analyse d'une séance qui ne s'est pas passée ainsi, et elle serait prise pour un verdict sur votre entraînement plutôt que sur un relevé incomplet.

Ce que le modèle reçoit, c'est la feuille de séance elle-même, le document que vous pouvez exporter : la prescription à gauche et ce que vous avez réellement fait à droite - répétitions, charge, RPE ressenti, 1RM estimé, et **votre propre commentaire sur chaque série**. Les séances déjà terminées du bloc l'accompagnent, pour que l'analyse puisse dire si vous progressez, stagnez ou dérivez, au lieu de décrire un jour isolé.

La réponse est **conservée** : elle coûte de l'argent, deux demandes donnent deux lectures différentes de la même séance, et vous devez pouvoir y revenir en mars.

Vous pouvez en demander **autant que vous voulez** : les analyses s'ajoutent
les unes aux autres au lieu de se remplacer - un deuxième avis d'un plus gros
modèle, ou l'analyse de votre coach à côté de la vôtre. Chacune indique
en dessous quelle souscription l'a produite et quand (« Généré par ma clé
Mistral le 19 octobre 2026 »), ce qui permet de les distinguer quand elles ne
disent pas la même chose.

Supprimer une analyse demande confirmation : elle a coûté de l'argent et ne
revient pas - il faudrait en redemander une.

### Votre coach la voit

Si le programme appartient à un club, l'analyse fait partie du dossier de la séance : votre coach la lit comme il lit vos commentaires.

Un coach peut aussi la **générer pour vous**, avec sa propre connexion IA. C'est ce qui permet à un seul abonnement de servir des athlètes qui n'en ont pas.

### Dans les exports

Une séance analysée emporte son analyse dans le **PDF, le tableur et l'image**, imprimée sous le tableau qu'elle commente.

## L'analyse de vos données fitness

Sur l'écran **Données fitness**, sous les graphiques, le même bouton lit ce que votre montre et votre balance ont enregistré : jusqu'à **quatre-vingt-dix jours** de pas et de fréquence cardiaque, et votre dernière composition corporelle.

Quatre-vingt-dix jours parce que la question est une direction plutôt qu'une journée : quelles semaines ont été chargées, lesquelles calmes, si les valeurs de repos dérivent.

Demandez-la **quand vous voulez** et, comme pour les boutons IA d'une séance,
c'est le bouton qui vous demande **quelle souscription** la produit lorsque
vous en avez plusieurs.

Chaque analyse **remplace** la précédente, au lieu de s'empiler comme celles
d'une séance : ce qu'elle décrit est une fenêtre qui se déplace, donc une
ancienne lecture est périmée plutôt qu'un second avis. La ligne en dessous
indique quelle souscription a produit celle que vous lisez, et quand.

Elle part aussi dans l'**export des données fitness**, PDF comme image, sous
les graphiques qu'elle commente - à condition d'avoir moins d'un jour. Une
analyse plus ancienne reste à l'écran avec sa date, mais n'entre pas dans le
document, où elle se lirait comme un commentaire de chiffres qu'elle n'a
jamais vus.

## Ce qui est dit au modèle

Les instructions sont des fichiers markdown sur le serveur, pas des chaînes enfouies dans le code : elles peuvent être améliorées sans toucher à l'application.

| Fichier | Ce qu'il gouverne |
| --- | --- |
| `coach.skill.md` | Comment un coach répond en général : le ton, quoi faire d'un RPE, ce qu'il ne faut jamais affirmer. |
| `feedback.skill.md` | Ce qu'est précisément une analyse de séance. |
| `health.skill.md` | Ce qu'est précisément une lecture de données fitness - y compris qu'il ne s'agit pas d'un diagnostic. |

Les fournisseurs eux-mêmes sont une table en base plutôt qu'une liste dans le
code : un exploitant peut corriger une adresse, ajouter un service compatible
OpenAI ou apprendre un nouveau nom de modèle au formulaire sans nouvelle
version.

Chacun est envoyé avec la question, le commun d'abord. La réponse est toujours demandée dans la langue configurée sur votre compte, en anglais à défaut.

## Ce que ce n'est pas

Le modèle n'est ni médecin ni votre coach. Il lit ce qu'on lui met sous les yeux - une feuille de séries, une série de comptes de pas - et dit ce qu'un coach bien informé en dirait. Il ignore le genou dont vous n'avez pas parlé, et il ne doit jamais être le seul lecteur de votre entraînement.
