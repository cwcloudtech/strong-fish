---
id: steps
title: Les pas de votre montre
sidebar_position: 6
---

StrongFish peut afficher les pas que votre montre a déjà comptés, à côté du
reste de votre entraînement. Il les lit ; il n'écrit jamais rien en retour, et
n'en garde aucune copie — les chiffres viennent de votre compte à chaque
consultation.

## Ce à quoi il se connecte

**Google Health**, où vivent désormais les données Fitbit. L'API développeur de
Fitbit a été coupée le 30 octobre 2026, et une Fitbit, une Pixel Watch ou un
téléphone qui compte les pas alimentent tous le même profil Google Health.

Le compte que vous connectez est donc votre **compte Google**, et ce qu'il lit
est ce que vos appareils ont déposé dans Google Health.

## Avant de connecter

Votre montre doit déjà être liée à Google Health. Pour une Fitbit, cela veut
dire ouvrir une fois l'**application Google Health** et y lier votre compte
Fitbit à votre compte Google.

C'est le piège classique : l'écran de consentement Google accepte volontiers un
compte sans profil Google Health derrière, et rien n'arriverait jamais.
StrongFish le vérifie au moment de la connexion et vous le dit, plutôt que de
vous laisser avec un graphique vide.

## Se connecter

1. Ouvrez les **Paramètres** sur le site, ou votre **profil** sur le téléphone.
2. Sous _Appareils connectés_, choisissez **Connecter** en face de Google Health.
3. Google vous demande de vous identifier et d'autoriser StrongFish à lire
   votre activité.
4. Vous revenez sur StrongFish, connecté.

Une seule permission est demandée : lire l'activité et la condition physique.
Ni votre sommeil, ni votre poids, et rien du tout en écriture.

Sur le téléphone, le consentement s'ouvre dans votre navigateur plutôt que dans
l'application — ainsi vous voyez sur quelle page de connexion vous tapez.
Revenez ensuite dans l'application, la connexion y est.

## Ce que vous voyez

Les pas par jour, dans la fenêtre de votre choix :

| Vue | Ce qu'elle couvre |
| --- | --- |
| Dernières 24h | Le compte d'aujourd'hui |
| Dernières 48h | Aujourd'hui et hier |
| Dernière semaine | Sept jours, avec la moyenne par 24h |
| Dernier mois | Trente jours, avec la moyenne par 24h |
| 90 derniers jours | Quatre-vingt-dix jours, avec la moyenne par 24h |

Ce sont des jours entiers, pas des fenêtres glissantes : « dernières 24h » est
le total d'aujourd'hui, le même nombre que celui affiché par la montre.
Quatre-vingt-dix jours est la vue la plus large qui existe, parce que c'est la
plage la plus longue que Google renvoie en une seule requête.

Chaque jour est coloré selon l'échelle de marche habituelle — moins de 6 000
pas, moins de 10 000, moins de 15 000, et au-delà — et la valeur par 24h
également. Les totaux sur plusieurs jours restent neutres : un mois de marche
atteint un chiffre qui serait dans la tranche haute quoi que vous ayez fait.

## Se déconnecter

**Déconnecter**, au même endroit, retire l'accès de StrongFish de son côté, et
les chiffres cessent d'être lus.

Cela ne révoque rien dans votre compte Google — c'est à vous de le faire,
depuis [la page des autorisations de votre compte Google](https://myaccount.google.com/permissions),
et StrongFish se garde délibérément d'aller le faire à votre place.

## Pour qui administre le serveur

Elle réutilise le client OAuth qui sert déjà à la connexion Google —
`SF_OIDC_GOOGLE_CLIENT_ID` et `SF_OIDC_GOOGLE_CLIENT_SECRET`. Pas de second
jeu d'identifiants : c'est le même projet Google Cloud, et un client OAuth
demande des permissions différentes selon la requête — une identité quand
quelqu'un se connecte, l'activité quand il relie une montre.

Quatre choses doivent être vraies de ce projet. L'ancienne page unique « OAuth
consent screen » n'existe plus : elle est éclatée en trois pages sous **Google
Auth Platform**, d'où les liens directs ci-dessous.

1. L'**API Google Health** y est activée :
   [bibliothèque d'API](https://console.cloud.google.com/apis/library/health.googleapis.com).
2. `<URL de votre API>/v1/devices/google-health/callback` figure parmi les URI
   de redirection autorisées du client OAuth :
   [Identifiants](https://console.cloud.google.com/apis/credentials). La
   connexion et le lien d'une montre reviennent sur des routes différentes :
   ajouter celle-ci est une étape à part entière.
3. Le scope Google Health est déclaré dans
   [Data Access](https://console.cloud.google.com/auth/scopes) — *Add or remove
   scopes*, puis chercher Google Health API.
4. Les personnes qui testent sont dans la liste des testeurs :
   [Audience](https://console.cloud.google.com/auth/audience), où *Publishing
   status* indique **Testing** et *User type* **External**. Ajoutez chaque
   adresse sous **Test users**.

Si la connexion Google n'est pas configurée du tout, la section apparaît quand
même, signalée comme non activée ici — le membre voit que la fonctionnalité
existe plutôt que de se demander.

Deux points à anticiper tant que l'app est en Testing :

- Elle est plafonnée à **100 testeurs**, ajoutés un par un.
- L'autorisation d'un testeur **expire sept jours après son consentement** : la
  connexion saute donc chaque semaine jusqu'à la publication. StrongFish le dit
  clairement — « la connexion a expiré, reconnectez le compte » — plutôt que
  d'afficher un graphique vide.

Les deux limites disparaissent une fois l'app publiée et validée. Dans une
organisation Google Workspace, un administrateur qui marque l'app **Trusted**
les lève pour les utilisateurs de son organisation.
