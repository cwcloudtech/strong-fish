---
id: health-data
title: Données santé
sidebar_position: 6
---

StrongFish peut afficher ce que votre montre et votre balance ont déjà mesuré - pas, fréquence cardiaque, poids, masse grasse, masse musculaire et le reste - à côté de votre entraînement. Il les lit ; il n'écrit jamais rien en retour, et n'en garde aucune copie : les chiffres viennent de vos comptes à chaque consultation.

## Ce à quoi il se connecte

| Compte | Ce qu'il lit |
| --- | --- |
| **Google Health** (Fitbit, Pixel Watch, téléphone) | Pas et fréquence cardiaque |
| **Withings** (balances, montres) | Composition corporelle, ainsi que pas et fréquence cardiaque |

**Google Health** est l'endroit où vivent désormais les données Fitbit. L'API développeur de Fitbit a été coupée le 30 octobre 2026, et une Fitbit, une Pixel Watch ou un téléphone qui compte les pas alimentent tous le même profil Google Health. Le compte que vous connectez est donc votre **compte Google**, et ce qu'il lit est ce que vos appareils ont déposé dans Google Health.

**Withings** se connecte avec votre compte Withings, celui auquel votre balance se synchronise.

Vous pouvez connecter l'un, l'autre, ou les deux.

### Quand les deux sont connectés

Les pas et la fréquence cardiaque viennent d'**un seul compte à la fois**, pour qu'un même jour ne soit jamais compté deux fois :

- Google Health est utilisé dès qu'il est connecté.
- Withings n'est utilisé pour les pas et la fréquence cardiaque que si aucun compte Google Health n'est connecté.

La composition corporelle vient toujours de Withings, quel que soit le compte d'où viennent les pas. La ligne Withings le signale quand ses pas ne sont pas ceux affichés.

## Avant de connecter

**Pour Google Health**, votre montre doit déjà être liée à Google Health. Pour une Fitbit, cela veut dire ouvrir une fois l'**application Google Health** et y lier votre compte Fitbit à votre compte Google.

C'est le piège classique : l'écran de consentement Google accepte volontiers un compte sans profil Google Health derrière, et rien n'arriverait jamais. StrongFish le vérifie au moment de la connexion et vous le dit, plutôt que de vous laisser avec un graphique vide.

**Pour Withings**, votre balance doit simplement être configurée dans l'application Withings, comme d'habitude.

## Se connecter

1. Ouvrez **Données santé** sur le site ou le téléphone.
2. Choisissez **Connecter** en face de Google Health ou de Withings.
3. Le fournisseur vous demande de vous identifier et d'autoriser StrongFish à lire vos données.
4. Vous revenez sur StrongFish, connecté.

Seules des permissions de lecture sont demandées :

- **Google Health** : l'activité et la condition physique, et la fréquence cardiaque. Ni votre sommeil, ni votre poids, et rien du tout en écriture.
- **Withings** : vos mesures et votre activité quotidienne.

Sur le téléphone, le consentement s'ouvre dans votre navigateur plutôt que dans l'application - ainsi vous voyez sur quelle page de connexion vous tapez. Revenez ensuite dans l'application, la connexion y est.

## Ce que vous voyez

### Pas

Les pas par jour, dans la fenêtre de votre choix :

| Vue | Ce qu'elle couvre |
| --- | --- |
| Dernières 24h | Le compte d'aujourd'hui |
| Dernières 48h | Aujourd'hui et hier |
| Dernière semaine | Sept jours, avec la moyenne par 24h |
| 14 derniers jours | Quatorze jours, avec la moyenne par 24h |
| Dernier mois | Trente jours, avec la moyenne par 24h |
| 90 derniers jours | Quatre-vingt-dix jours, avec la moyenne par 24h |

Ce sont des jours entiers, pas des fenêtres glissantes : « dernières 24h » est le total d'aujourd'hui, le même nombre que celui affiché par la montre. Quatre-vingt-dix jours est la vue la plus large qui existe, parce que c'est la plage la plus longue que Google renvoie en une seule requête.

Chaque jour est coloré selon l'échelle de marche habituelle - moins de 6 000 pas, moins de 10 000, moins de 15 000, et au-delà - et la valeur par 24h également. Les totaux sur plusieurs jours restent neutres : un mois de marche atteint un chiffre qui serait dans la tranche haute quoi que vous ayez fait.

### Fréquence cardiaque

Sur les mêmes jours, une barre par jour aussi haute que la fréquence la plus élevée, colorée selon la zone la plus intense atteinte : bleu pour légère, jaune pour modérée, orange pour intense, rouge pour maximale. Les zones sont celles de votre fournisseur, calculées à partir de votre propre fréquence cardiaque maximale.

Google Health ne remonte que **14 jours** pour la fréquence cardiaque. Sur les vues plus larges, les pas restent affichés en entier, avec votre dernière fréquence relevée, mais sans graphique de fréquence cardiaque qui couvrirait moins que ce que son titre annonce.

### Composition corporelle

Avec Withings connecté, un panneau **Composition corporelle** affiche la dernière valeur de chaque mesure que prend votre balance, chacune avec sa date :

| Mesure | Unité |
| --- | --- |
| Poids | kg |
| Masse grasse | % et kg |
| Masse maigre | kg |
| Masse musculaire | kg |
| Eau corporelle | kg |
| Masse osseuse | kg |
| Graisse viscérale | indice |
| Métabolisme de base | kcal |
| IMC | kg/m² (depuis votre dernier poids et la taille de votre profil Withings) |

Une mesure que votre balance ne prend pas n'apparaît simplement pas. Chacune a sa propre date, car une balance ne prend pas toutes les mesures à chaque fois - la masse grasse, par exemple, ne se lit que pieds nus.

#### Estimation du mode athlète

Le **mode athlète** de Withings, prévu pour les personnes qui s'entraînent plus de huit heures par semaine, a tendance à sous-estimer la masse grasse. À côté des mesures de masse grasse et de masse maigre, le panneau affiche donc entre parenthèses ce qu'elles seraient avec **5 points de masse grasse en plus** :

```
Masse grasse (%)   12 % (~ 17 %)
Masse grasse       9.6 kg (~ 13.6 kg)
Masse maigre       70.4 kg (~ 66.4 kg)
Masse musculaire   66.9 kg (~ 62.9 kg)
```

Les kilos bougent de 5 % de votre poids, pour que masse grasse et masse maigre en fassent toujours la somme. Le poids, la masse osseuse, l'eau corporelle, la graisse viscérale, le métabolisme et l'IMC restent tels que mesurés.

Withings n'indique pas aux autres applications si le mode athlète est activé : l'estimation est donc toujours affichée. C'est une approximation, pas une correction publiée par Withings : si votre balance est en mode standard, lisez-la comme le haut de la fourchette.

## Export

**Exporter cette vue** enregistre ce qui est à l'écran en PNG ou en PDF : les pas et la fréquence cardiaque tracés en histogrammes, aux couleurs de l'écran, suivis d'un tableau de votre dernière composition corporelle quand Withings est connecté.

## Se déconnecter

**Déconnecter**, au même endroit, retire l'accès de StrongFish de son côté, et les chiffres cessent d'être lus.

Cela ne révoque rien dans votre compte chez le fournisseur - c'est à vous de le faire, depuis [la page des autorisations de votre compte Google](https://myaccount.google.com/permissions) ou les applications connectées de votre compte Withings, et StrongFish se garde délibérément d'aller le faire à votre place.

## Pour qui administre le serveur

Une intégration dont les identifiants ne sont pas renseignés apparaît quand même, signalée comme non activée ici - le membre voit que la fonctionnalité existe plutôt que de se demander.

### Google Health

Elle réutilise le client OAuth qui sert déjà à la connexion Google - `SF_OIDC_GOOGLE_CLIENT_ID` et `SF_OIDC_GOOGLE_CLIENT_SECRET`. Pas de second jeu d'identifiants : c'est le même projet Google Cloud, et un client OAuth demande des permissions différentes selon la requête - une identité quand quelqu'un se connecte, l'activité quand il relie une montre.

Quatre choses doivent être vraies de ce projet. L'ancienne page unique « OAuth consent screen » n'existe plus : elle est éclatée en trois pages sous **Google Auth Platform**, d'où les liens directs ci-dessous.

1. L'**API Google Health** y est activée : [bibliothèque d'API](https://console.cloud.google.com/apis/library/health.googleapis.com).
2. `<URL de votre API>/v1/devices/google-health/callback` figure parmi les URI de redirection autorisées du client OAuth : [Identifiants](https://console.cloud.google.com/apis/credentials). La connexion et le lien d'une montre reviennent sur des routes différentes : ajouter celle-ci est une étape à part entière.
3. Le scope Google Health est déclaré dans [Data Access](https://console.cloud.google.com/auth/scopes) - *Add or remove scopes*, puis chercher Google Health API.
4. Les personnes qui testent sont dans la liste des testeurs : [Audience](https://console.cloud.google.com/auth/audience), où *Publishing status* indique **Testing** et *User type* **External**. Ajoutez chaque adresse sous **Test users**.

Deux points à anticiper tant que l'app est en Testing :

- Elle est plafonnée à **100 testeurs**, ajoutés un par un.
- L'autorisation d'un testeur **expire sept jours après son consentement** : la connexion saute donc chaque semaine jusqu'à la publication. StrongFish le dit clairement - « la connexion a expiré, reconnectez le compte » - plutôt que d'afficher un graphique vide.

Les deux limites disparaissent une fois l'app publiée et validée. Dans une organisation Google Workspace, un administrateur qui marque l'app **Trusted** les lève pour les utilisateurs de son organisation.

### Withings

1. Créez une application dans le [tableau de bord développeur Withings](https://developer.withings.com/dashboard/).
2. Déclarez `<URL de votre API>/v1/devices/withings/callback` comme URL de callback.
3. Renseignez `SF_WITHINGS_CLIENT_ID` et `SF_WITHINGS_CLIENT_SECRET`.
