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
| **Stance Fitness** (capteur de vitesse de barre) | La vitesse de barre et le RPE mesuré sur votre dernière série de squat, développé couché et soulevé de terre |

**Google Health** est l'endroit où vivent désormais les données Fitbit. L'API développeur de Fitbit a été coupée le 30 octobre 2026, et une Fitbit, une Pixel Watch ou un téléphone qui compte les pas alimentent tous le même profil Google Health. Le compte que vous connectez est donc votre **compte Google**, et ce qu'il lit est ce que vos appareils ont déposé dans Google Health.

**Withings** se connecte avec votre compte Withings, celui auquel votre balance se synchronise.

**Stance Fitness** est un capteur de vitesse de barre - un petit accéléromètre qui se fixe sur la barre. Vous l'associez à votre compte Stance, et le suivi lui-même se fait dans l'application Stance sur votre téléphone, là où la barre est réellement mesurée. StrongFish relit votre dernière série de chaque mouvement, et sur le téléphone il peut ouvrir l'application Stance pour que vous enregistriez la suivante.

Vous pouvez connecter n'importe lesquels, dans n'importe quelle combinaison.

### Quand les deux sont connectés

Les pas et la fréquence cardiaque viennent d'**un seul compte à la fois**, pour qu'un même jour ne soit jamais compté deux fois :

- Google Health est utilisé dès qu'il est connecté.
- Withings n'est utilisé pour les pas et la fréquence cardiaque que si aucun compte Google Health n'est connecté.

La composition corporelle vient toujours de Withings, quel que soit le compte d'où viennent les pas. La ligne Withings le signale quand ses pas ne sont pas ceux affichés.

## Avant de connecter

**Pour Google Health**, votre montre doit déjà être liée à Google Health. Pour une Fitbit, cela veut dire ouvrir une fois l'**application Google Health** et y lier votre compte Fitbit à votre compte Google.

C'est le piège classique : l'écran de consentement Google accepte volontiers un compte sans profil Google Health derrière, et rien n'arriverait jamais. StrongFish le vérifie au moment de la connexion et vous le dit, plutôt que de vous laisser avec un graphique vide.

**Pour Withings**, votre balance doit simplement être configurée dans l'application Withings, comme d'habitude.

**Pour Stance**, installez l'application Stance sur le même téléphone et connectez-vous-y une fois. C'est ce compte que vous associez, et c'est l'application Stance qui capte la barre - ce doit donc être le téléphone avec lequel vous soulevez.

## Se connecter

1. Ouvrez **Données santé** sur le site ou le téléphone.
2. Choisissez **Connecter** en face de Google Health ou de Withings.
3. Le fournisseur vous demande de vous identifier et d'autoriser StrongFish à lire vos données.
4. Vous revenez sur StrongFish, connecté.

Seules des permissions de lecture sont demandées :

- **Google Health** : l'activité et la condition physique, et la fréquence cardiaque. Ni votre sommeil, ni votre poids, et rien du tout en écriture.
- **Withings** : vos mesures et votre activité quotidienne.

**Stance** fait exception à ce ballet de connexion. Vous pouvez le relier de deux façons : en tapant votre e-mail et votre mot de passe Stance directement dans StrongFish (utilisés une fois pour un jeton, jamais conservés), ou via **Se connecter avec Google** avec le compte Google que votre compte Stance utilise. Sur le téléphone, l'étape Google s'ouvre dans l'app web, où tourne la connexion Google.

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

### Vitesse de barre

Avec Stance associé, un panneau **Vitesse de barre** affiche votre dernière série de chacun des trois mouvements - squat, développé couché et soulevé de terre. Chaque série est une rangée de barres, une par répétition, chacune aussi haute que la vitesse de barre de cette répétition, au-dessus de la charge, des répétitions et des deux RPE : celui que vous avez donné et celui que Stance a mesuré à partir de la barre.

Seule la série la plus récente de chaque mouvement est montrée - ce qui compte est la forme de votre dernier effort, pas un historique. Enregistrez une nouvelle série et elle prend la place de l'ancienne, et un mouvement que vous n'avez pas suivi n'apparaît tout simplement pas.

Sur le téléphone, un bouton **Enregistrer une série** ouvre l'application Stance pour suivre la série suivante ; revenez ensuite et elle y est. Le site n'a pas ce bouton - le téléphone est l'endroit où la barre est mesurée - mais une série enregistrée sur le téléphone apparaît aussi sur le site.

### Comment le RPE mesuré est calculé

Le RPE mesuré vient de la vitesse de la barre : plus une répétition est lente par rapport à la vitesse à laquelle cette charge monte d'habitude chez vous, plus la série est proche de l'échec. Stance le calcule à partir de votre série, et StrongFish lit et trace le résultat - il ne fait aucune estimation lui-même. Pour la méthode derrière, écrite comme de la science du sport et non comme quelque chose de propre à un traqueur en particulier, voir **[L'effort basé sur la vitesse](./velocity-based-rpe.md)**.

## Export

**Exporter cette vue** enregistre ce qui est à l'écran en PNG ou en PDF : les pas et la fréquence cardiaque tracés en histogrammes, aux couleurs de l'écran, suivis d'un tableau de votre dernière composition corporelle quand Withings est connecté, et des mêmes graphiques de vitesse de barre pour vos derniers squat, développé couché et soulevé de terre quand Stance est associé.

## Se déconnecter

**Déconnecter**, au même endroit, retire l'accès de StrongFish de son côté, et les chiffres cessent d'être lus.

Cela ne révoque rien dans votre compte chez le fournisseur - c'est à vous de le faire, depuis [la page des autorisations de votre compte Google](https://myaccount.google.com/permissions) ou les applications connectées de votre compte Withings, et StrongFish se garde délibérément d'aller le faire à votre place. Pour Stance, il n'y a pas de page de ce genre à visiter - se déconnecter supprime le jeton que StrongFish détient, et c'est tout.

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

### Stance Fitness

L'intégration Stance fonctionne avec des valeurs fournies par Stance. Écrivez à leur équipe à **hello@stancefitness.co** pour demander un accès, puis renseignez ce qu'ils vous transmettent :

- `SF_STANCE_FIREBASE_API_KEY`, `SF_STANCE_SUPABASE_PROJECT_REF` et `SF_STANCE_SUPABASE_ANON_KEY` - la connexion au compte et les séries stockées. **Requis** : si l'une est vide, Stance apparaît comme non activé ici, comme toute autre intégration sans ses identifiants.
- `SF_STANCE_GOOGLE_CLIENT_ID` - l'identifiant client Google public, pour proposer **Se connecter avec Google** à côté du formulaire e-mail/mot de passe. Facultatif, et sans secret : c'est une valeur publique comme la clé anon. Pour le bouton web, Stance déclare l'origine de votre app web dans les origines JavaScript autorisées de ce client.
- `SF_STANCE_USERS_TABLE`, `SF_STANCE_SESSIONS_RPC`, `SF_STANCE_APP_URL` - surcharges pour la relation utilisateur, la RPC des séries et ce qu'ouvre le bouton **Enregistrer une série**. Facultatif ; les valeurs par défaut sont celles de Stance.
