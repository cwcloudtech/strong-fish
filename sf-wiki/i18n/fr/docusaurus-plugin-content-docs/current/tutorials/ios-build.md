---
id: ios-build
title: Tester l'application iOS
sidebar_position: 5
---

Il n'y a pas encore de version sur l'App Store, et iOS n'a pas d'équivalent au téléchargement d'un APK — Apple n'autorise pas l'installation d'une application depuis un lien web. Le chemin vers un téléphone passe donc par **TestFlight**, l'application de test d'Apple.

Cette page s'adresse à qui construit et distribue le build. Si vous voulez
simplement installer l'application, allez directement à [Installer un build](#installer-un-build).

## Pourquoi celui-ci est fabriqué à la main

Tous les autres artefacts de StrongFish sont produits par le pipeline. iOS ne peut pas l'être : `flutter build ipa` appelle `xcodebuild` et `codesign`, tous deux réservés à macOS, et la licence d'Apple garde macOS sur du matériel Apple — il n'existe donc aucune image de runner capable de le faire.

La jointure est un script, [`ci/app/deliver-ios.sh`](https://gitlab.cwcloud.tech/oss/strong-fish/-/blob/main/ci/app/deliver-ios.sh),
lancé à la main sur un Mac. Il construit et signe l'`.ipa` et le dépose dans `sf-mobile/dist/`, qui est versionné ; le pipeline Linux copie ensuite ce qui s'y trouve dans l'image `ui-and-mobile`. Quand personne n'a rien construit, le dossier ne contient que son fichier témoin et le pipeline continue normalement.

## Ce qu'il faut, une fois

- Un Mac avec **Xcode**, installé depuis l'App Store. Le script ne peut pas l'installer à votre place : il pèse plusieurs gigaoctets et seule une personne peut en accepter la licence. Tout le reste (Flutter, CocoaPods), il l'installe via Homebrew s'il manque.
- **Homebrew**, si Flutter ou CocoaPods ne sont pas déjà installés. Le script n'installe pas Homebrew lui-même : son installateur est un script téléchargé puis exécuté avec sudo, ce qu'un script de build n'a pas à faire à votre place.
- Une adhésion à l'**Apple Developer Program** (99 €/an). Un compte gratuit permet de construire pour son propre appareil pendant sept jours, mais pas d'utiliser TestFlight.
- L'application déclarée dans App Store Connect sous l'identifiant `tech.cwcloud.strongFishMobile`.

Puis, dans le dépôt :

```bash
cp .env.ios.example .env.ios
```

Il n'y a qu'une chose à renseigner : `IOS_TEAM_ID`. Tout le reste est soit
fabriqué pour vous, soit fixé dans le projet Xcode, soit une valeur par défaut du script.

### Le certificat de signature

Au premier lancement, s'il n'y a pas d'`ios.p12`, le script prend l'identité
**Apple Distribution** qu'Xcode a déjà placée dans votre trousseau, l'écrit dans `ios.p12`, et ajoute à `.env.ios` le mot de passe qu'il a inventé. C'est ensuite ce fichier qui signe : chaque exécution l'importe dans un trousseau dédié, de sorte que le build dépend du certificat et non de ce que la machine a accumulé par ailleurs.

Il affiche ce qui s'est retrouvé dans le fichier, car `security export` ne sait pas sélectionner une identité en particulier : si votre trousseau en contient plusieurs, elles y passent toutes, et mieux vaut le savoir avant de copier le fichier ailleurs.

S'il n'existe pas encore d'identité de distribution, il indique où en créer une : **Xcode → Réglages → Comptes → votre identifiant Apple → Gérer les certificats → + → Apple Distribution**. Un certificat de *développement* est volontairement refusé : il ne peut pas signer un build pour TestFlight, et le choisir échouerait bien plus tard avec un message nettement moins clair.

`ios.p12` et le mot de passe dans `.env.ios` sont tous deux ignorés par git, et vont ensemble : perdre l'un revient à supprimer les deux et à laisser l'exécution suivante en fabriquer de nouveaux.

## Construire

```bash
./ci/app/deliver-ios.sh
```

Le script lit la version dans `VERSION`, le même fichier que le build Android et le manifeste de l'API, et en dérive le numéro de build de la même façon : `1.2.3` devient `10203`. Cela compte davantage sur iOS que sur Android : App Store Connect refuse un envoi dont le numéro de build n'est pas supérieur au précédent, et c'est donc l'incrément de `VERSION` qui rend l'envoi suivant possible.

Le résultat arrive dans `sf-mobile/dist/strong-fish-v<VERSION>.ipa`. Commitez-le et poussez : le pipeline l'ajoute à l'image.

## L'envoyer sur TestFlight

```bash
./ci/app/deliver-ios.sh --upload
```

Cela nécessite une clé d'API App Store Connect, depuis **App Store Connect → Utilisateurs et accès → Intégrations**. Mettez `IOS_API_KEY_ID` et `IOS_API_ISSUER_ID` dans `.env.ios`, et déposez le fichier `.p8` lui-même à la racine du dépôt sous le nom donné par Apple — `AuthKey_<id de la clé>.p8`. Il est ignoré par git et y reste.

Apple ne laisse télécharger ce fichier qu'une seule fois : c'est pourquoi le script le garde comme fichier plutôt que de vous demander de le recopier.

Apple met quelques minutes à traiter un envoi. Le build apparaît ensuite dans **TestFlight** sur App Store Connect, où vous ajoutez des testeurs :

- **Testeurs internes** — jusqu'à 100 personnes de votre équipe App Store Connect. Ils reçoivent le build immédiatement, sans validation.
- **Testeurs externes** — jusqu'à 10 000, par e-mail ou par lien public. Le *premier* build d'un groupe passe une courte Beta App Review, en général un jour environ.

Le test interne est la voie la plus rapide tant que l'application bouge encore.

## Installer un build {#installer-un-build}

1. Installez **TestFlight** depuis l'App Store.
2. Ouvrez l'invitation — l'e-mail envoyé par Apple ou le lien de test.
3. Acceptez, puis installez StrongFish depuis TestFlight.

Un build TestFlight cesse de fonctionner au bout de **90 jours**. C'est une règle d'Apple, pas la nôtre : à l'expiration, installez le build suivant de la même façon.

## Ce qui diffère de l'application Android

**Pas de bouton de mise à jour.** L'application Android interroge
`/v1/mobile-app` et sait installer elle-même un APK plus récent. iOS ne le peut pas : une application qui installe son propre code est refusée à la validation, et il n'existe pas de solution de repli par installation manuelle. Sur iOS, l'écran des réglages indique que les mises à jour arrivent par l'App Store, et TestFlight prévient quand un nouveau build est disponible.

**L'`.ipa` publié sur le site ne s'installe pas en cliquant dessus.** Il est publié pour que le build signé soit exactement celui qui est distribué, et pour pouvoir être fourni à un outil de gestion de flotte ou à une installation ad-hoc. Un téléphone ne peut pas l'installer depuis un lien.

## Le jour où il arrivera sur l'App Store

Rien ne change dans le build. TestFlight et la boutique prennent le même `.ipa` issu du même script ; ce qui change, c'est que vous le soumettez à l'App Review au lieu de le distribuer à des testeurs. Cette page sera remplacée par un lien vers la boutique à ce moment-là.
