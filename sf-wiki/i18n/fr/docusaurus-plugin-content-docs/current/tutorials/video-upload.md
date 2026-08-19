---
id: video-upload
title: Publier une vidéo
sidebar_position: 3
---

Vous pouvez joindre une vidéo à une publication — une tentative en compétition,
une vérification technique — mais elle part dans **votre propre stockage**, pas
dans le nôtre.

## Pourquoi votre propre bucket

StrongFish n'héberge aucune vidéo. Vingt mégaoctets par publication mettraient
la base de données à genoux, et payer pour diffuser les vidéos d'entraînement
des autres n'est pas la vocation de cette application.

Vous apportez donc une destination : un **bucket compatible S3** ou un **dossier
Google Drive**. L'application y dépose le fichier et la publication porte un lien
vers lui. Tant que vous n'en avez pas configuré, le bouton vidéo répond
« configurez d'abord votre stockage » — l'API renvoie un 405, c'est-à-dire que
la requête était correcte mais que la fonctionnalité n'est pas encore disponible
sur votre compte.

## Option 1 : un bucket compatible S3

Fonctionne avec AWS S3, MinIO, Scaleway, DigitalOcean Spaces — tout ce qui parle
l'API S3.

1. Créez un bucket, et une clé d'accès autorisée à y écrire.
2. **Les objets doivent être lisibles publiquement.** Le lien part dans une
   publication et est lu par un lecteur vidéo dans le navigateur de quelqu'un
   d'autre, sans aucun justificatif. StrongFish dépose chaque objet avec une ACL
   `public-read` ; un bucket dont les ACL sont désactivées refusera l'envoi, et
   c'est le bon moment pour s'en apercevoir.
3. Dans l'application : **Paramètres → Stockage des vidéos**, choisissez *Bucket
   compatible S3*.
4. Renseignez :

   | Champ | Exemple |
   | --- | --- |
   | Endpoint | `https://s3.eu-west-3.amazonaws.com` |
   | Région | `eu-west-3` |
   | Bucket | `mes-videos-powerlifting` |
   | Clé d'accès / Clé secrète | fournies par votre hébergeur |
   | Sous-dossier *(facultatif)* | `strong-fish` |
   | Adresse publique *(facultatif)* | votre CDN ou domaine personnalisé, si le bucket est servi par l'un des deux |

5. Enregistrez.

![Le stockage vidéo configuré sur un bucket S3](/img/screenshots/video-storage.png)

## Option 2 : un dossier Google Drive

1. Dans la console Google Cloud, créez un **compte de service** et téléchargez
   sa **clé JSON**.
2. Créez (ou choisissez) un dossier Drive, et **partagez-le avec l'adresse
   e-mail du compte de service** en droits d'écriture. Sans cela il ne pourra
   rien y écrire — c'est l'étape que tout le monde oublie.
3. Copiez l'identifiant du dossier : c'est la dernière partie de son URL,
   `https://drive.google.com/drive/folders/<cette partie>`.
4. Dans l'application : **Paramètres → Stockage des vidéos**, choisissez
   *Dossier Google Drive*.
5. Téléversez le fichier de clé JSON et collez l'identifiant du dossier.
6. Indiquez éventuellement un **sous-dossier** — `strong-fish/videos`, par
   exemple. Il est créé dans le dossier partagé s'il n'existe pas encore : vous
   n'avez pas à le créer à la main. Laissez vide pour écrire directement dans le
   dossier.
7. Enregistrez.

StrongFish accorde à chaque fichier déposé un accès en lecture « toute personne
disposant du lien » au moment de l'écriture, et publie le lecteur Drive.

## Publier

1. Allez dans le fil et commencez une publication.
2. Choisissez **Ajouter une vidéo** et sélectionnez un fichier — MP4, WebM ou
   MOV, jusqu'à 20 Mo par défaut.
3. L'URL du fichier déposé est ajoutée au texte de votre publication.
4. Écrivez ce que vous voulez autour, et publiez.

Le lecteur apparaît automatiquement. Il n'y a pas de champ « lien » séparé : **la
première URL d'une publication est son média**, que vous l'ayez déposée ou collée
depuis YouTube.

## En cas de problème

| Ce que vous voyez | Ce que c'est généralement |
| --- | --- |
| « Configurez votre propre stockage vidéo » | Aucun stockage configuré. |
| « Votre stockage a refusé cet envoi » | Mauvaises clés, mauvais nom de bucket, ou ACL désactivées. |
| « Cette vidéo est trop volumineuse » | Au-delà de la limite — l'écran de stockage vous indique laquelle. |
| « Pas une vidéo lisible par un navigateur » | Réencodez en MP4 (H.264). |
| La publication affiche une carte de lien, pas un lecteur | L'URL du fichier n'est pas publiquement lisible. Vérifiez la politique du bucket, ou le partage du dossier Drive. |
