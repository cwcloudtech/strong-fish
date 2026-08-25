---
id: video-upload
title: Vidéos et messages vocaux
sidebar_position: 3
---

Les liens vidéo de YouTube, Vimeo et consorts se lisent tout seuls dans le fil :
collez-en un dans une publication et le lecteur apparaît. Vous pouvez aussi
joindre directement un fichier vidéo — une tentative en compétition, une
vérification technique —, en envoyer un en message privé, ou enregistrer un
message vocal dans une conversation. Ces trois-là partent dans **votre propre
stockage**, pas dans le nôtre.

## Pourquoi votre propre bucket

StrongFish n'héberge ni vidéo ni audio, parce que nous voulons rester gratuits et open source — et cela implique d'accepter certains compromis pour réduire les coûts.

Vous pouvez donc configurer donc une destination : un **bucket compatible S3** ou un **dossier Google Drive**. L'application y dépose le fichier et la publication porte un lien vers lui. Tant que vous n'en avez pas configuré, les boutons vidéo et microphone
répondent « configurez d'abord votre stockage » — l'API renvoie un 405, c'est-à-dire que la requête était correcte mais que la fonctionnalité n'est pas
encore disponible sur votre compte.

Configurez-le une fois et il couvre les trois usages : vidéos dans les publications, vidéos dans les messages, et messages vocaux.

## Option 1 : un bucket compatible S3

Fonctionne avec AWS S3, MinIO, Scaleway, DigitalOcean Spaces — tout ce qui est compatible avec l'API S3 (object storage).

1. Créez un bucket, et une clé d'accès autorisée à y écrire.
2. **Le bucket n'a pas besoin d'être public.** StrongFish sert lui-même les fichiers d'un bucket, avec vos identifiants : rien n'a besoin d'être lisible sans eux. Par défaut il demande quand même une ACL `public-read` à l'écriture (pratique si vous servez aussi le bucket via un CDN) ; si votre bucket l'interdit — c'est fréquent — activez **Ce bucket n'est pas public** et l'envoi cesse de la demander.
3. Dans l'application : **Paramètres → Stockage des vidéos**, choisissez *Bucket compatible S3*.
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

![Le stockage vidéo configuré sur un bucket S3](../../../../../static/img/screenshots/video-storage.png)

## Option 2 : un dossier Google Drive

1. Dans la console Google Cloud, créez un [**compte de service**](https://docs.cloud.google.com/iam/docs/service-account-overview) et téléchargez sa **clé JSON**.
2. Créez (ou choisissez) un dossier Drive, et **partagez-le avec l'adresse e-mail du compte de service** en droits d'écriture. Sans cela il ne pourra rien y écrire — c'est l'étape que tout le monde oublie.
3. Copiez l'identifiant du dossier : c'est la dernière partie de son URL, `https://drive.google.com/drive/folders/<cette partie>`.
4. Dans l'application : **Paramètres → Stockage des vidéos**, choisissez *Dossier Google Drive*.
5. Téléversez le fichier de clé JSON et collez l'identifiant du dossier.
6. Indiquez éventuellement un **sous-dossier** — `strong-fish/videos`, par exemple. Il est créé dans le dossier partagé s'il n'existe pas encore : vous n'avez pas à le créer à la main. Laissez vide pour écrire directement dans le dossier.
7. Enregistrez.

![drive-sa](../../../../../static/img/screenshots/drive-sa.png)

StrongFish accorde à chaque fichier déposé un accès en lecture « toute personne disposant du lien » au moment de l'écriture, et publie le lecteur Drive.

## D'où les fichiers sont lus

Les deux types de stockage se lisent différemment, et il vaut mieux savoir
lequel vous avez :

* **Un bucket est toujours servi par StrongFish lui-même.** Le lien de la
  publication est une adresse sur l'application, pas sur votre bucket :
  StrongFish récupère l'objet avec *vos* identifiants et le transmet au
  lecteur. Cela fonctionne que le bucket soit public ou non, et c'est bien
  l'intérêt — un bucket qui interdit les fichiers publics est la configuration
  normale en entreprise, et un lien qui ne marche que sur les buckets publics
  ne marche que par chance.
* **Un fichier Drive est lu depuis Drive.** L'envoi le partage avec toute
  personne détenant le lien, et la publication porte l'adresse `/preview` de
  Drive, que le lecteur intègre. Un fichier Drive est donc accessible à
  quiconque a le lien, et StrongFish ne peut pas restreindre cela. S'il vous
  faut des médias que seul votre club puisse voir, prenez un bucket.

**Qui peut regarder le fichier d'un bucket** suit la visibilité de votre profil
— la même règle qui décide si vos publications sont lisibles : tout le monde,
vos clubs, ou vos coachs (voir [créer un compte](./signup.md)). Les personnes
avec qui vous partagez le stockage le peuvent aussi. Les autres ne voient rien,
exactement comme si la publication ne contenait pas de vidéo.

**Ce bucket n'est pas public**, dans Paramètres → Stockage vidéo, ne contrôle
qu'une chose : si l'envoi demande un accès public au moment de l'écriture.
Activez-le si votre bucket refuse les fichiers publics. Cela ne change pas qui
peut regarder — c'est toujours la règle ci-dessus.

## Plusieurs stockages à la fois

Vous pouvez en configurer plusieurs, et l'ordre compte :

* **chaque envoi est déposé dans tous** — le même fichier, sous le même nom,
  dans chacun ;
* **le lien de la publication vient du premier**.

C'est ainsi qu'un club garde une seconde copie des vidéos de ses athlètes : le
bucket de la salle en premier, un bucket hors site en second. Si une
destination refuse l'envoi, la publication passe quand même par celles qui
l'ont accepté et l'échec vous est signalé — un bucket qui a cessé d'accepter
les fichiers ne fait donc pas échouer votre publication, mais ne reste pas
silencieux pour autant.

Utilisez les flèches dans **Paramètres → Stockage vidéo** pour changer le
premier. Un stockage ajouté se place en dernier : le promouvoir est une
décision, pas un effet de bord de sa configuration.

## Partager votre stockage

Un bucket coûte de l'argent et un club n'en a généralement qu'un. Dans
**Paramètres → Stockage vidéo**, ouvrez l'icône des personnes sur un stockage,
choisissez des membres et donnez-leur un rôle :

| Rôle | Ce qu'ils peuvent faire |
| --- | --- |
| Lire | Regarder ce qui se trouve dans ce stockage, même si votre profil ne le leur permettrait pas |
| Déposer et lire | Ce qui précède, et y déposer leurs propres vidéos |

Vous seul pouvez partager votre stockage, et vous seul pouvez arrêter de le
partager : une personne à qui vous donnez le droit d'écrire ne peut pas le
transmettre.

**Où vont vos propres envois :** dans vos stockages d'abord, dans votre ordre,
puis dans ceux partagés avec vous. Une personne qui n'en a aucun dépose dans le
premier stockage partagé avec elle — un athlète à qui son coach a prêté un
bucket peut donc publier une vidéo sans en posséder un.

## Publier une vidéo

1. Allez dans le fil et commencez une publication.
2. Choisissez **Ajouter une vidéo** et sélectionnez un fichier — MP4, WebM ou MOV, jusqu'à 20 Mo par défaut.
3. L'URL du fichier déposé est ajoutée au texte de votre publication.
4. Écrivez ce que vous voulez autour, et publiez.

Le lecteur apparaît automatiquement. Il n'y a pas de champ « lien » séparé : **la première URL d'une publication est son média**, que vous l'ayez déposée ou collée depuis YouTube.

Cela fonctionne de la même façon dans l'**application mobile** : l'icône caméra sous le champ de saisie prend une vidéo depuis votre téléphone.

## Envoyer une vidéo en message

Une conversation privée dispose des mêmes boutons : une image, une vidéo et un microphone. Une vidéo envoyée ainsi est déposée exactement comme celle d'une publication et ajoutée au message sous forme de lien, pour se lire dans la conversation.

## Enregistrer un message vocal

Appuyez sur le **microphone** dans une conversation pour démarrer l'enregistrement, et appuyez à nouveau pour l'arrêter. 

L'enregistrement est déposé à l'arrêt, pas au moment de l'envoi : le message part donc dès que vous appuyez sur envoyer.

L'application web comme le téléphone savent enregistrer. 
Sur téléphone, Android demande l'autorisation d'utiliser le microphone la première fois.

Un message vocal stocké sur **Google Drive** se lit dans le lecteur de Drive plutôt que dans une barre audio ordinaire — Drive sert une page d'aperçu pour un fichier, et non le fichier lui-même. Rien à configurer ; c'est bon à savoir pour que la différence entre les messages de deux membres ne ressemble pas à un bug.

## Du son dans une publication

Une publication lit le son comme elle lit la vidéo : collez le lien d'un fichier audio — un `.mp3`, un `.m4a`, un `.wav`, ou un enregistrement déposé dans votre stockage — et il devient une barre de lecture dans la publication plutôt qu'un simple lien. Les liens vers un fichier d'un stockage privé sont lus au travers de l'application : seuls les membres autorisés peuvent les écouter.

## En cas de problème

| Ce que vous voyez | Ce que c'est généralement |
| --- | --- |
| « Configurez votre propre stockage vidéo » | Aucun stockage configuré. Le même message vaut pour les vidéos et les messages vocaux. |
| « Votre stockage a refusé cet envoi » | Mauvaises clés, mauvais nom de bucket, ou ACL désactivées. |
| « Cette vidéo est trop volumineuse » | Au-delà de la limite — l'écran de stockage vous indique laquelle. |
| « Pas une vidéo lisible par un navigateur » | Réencodez en MP4 (H.264). |
| La publication affiche une carte de lien, pas un lecteur | L'URL du fichier n'est pas publiquement lisible. Vérifiez la politique du bucket, ou le partage du dossier Drive. |
| Le bouton microphone ne fait rien sur le téléphone | L'autorisation d'enregistrer a été refusée. Accordez-la dans les paramètres de l'application sous Android. |
