---
id: api
title: API RESTful
sidebar_position: 2
---

StrongFish dispose d'une API REST. Sa spécification OpenAPI est publiée [ici](https://api.strong-fish.com), et les requêtes s'y authentifient avec une clé d'API.

## Clés d'API {#api-keys}

### Ce qu'est une clé d'API

Un jeton qui authentifie une requête avec un en-tête `X-Api-Key` plutôt qu'avec une session. Elle porte **exactement vos droits** — ni plus, ni moins : traitez-la comme votre mot de passe.

L'API n'en stocke que l'empreinte. La clé elle-même n'existe qu'une fois, dans l'écran qui la crée, et ne peut plus être affichée ensuite. C'est délibéré : un jeton qu'un serveur peut relire est un jeton qu'un serveur compromis peut distribuer.

### En créer une, et s'en servir pour se connecter

Allez dans _Clés d'API_ dans le menu et créez-en une :

![api-key-create-1](../../../../../static/img/screenshots/api-key-create-1.png)

Vous pouvez ensuite copier la valeur affichée et la garder en lieu sûr, pour l'utiliser avec l'en-tête `X-Api-Key` dans votre script.

![api-key-create-2](../../../../../static/img/screenshots/api-key-create-2.png)

Ou afficher le QR code, avec l'icône en forme d'_œil_, si vous en avez besoin pour vous connecter sur mobile :

![api-key-create-3](../../../../../static/img/screenshots/api-key-create-3.png)

Attention : une fois la fenêtre fermée, **elle ne reviendra pas**.

### Révoquer

_Clés d'API_ liste celles que vous avez, leur date d'expiration et un bouton de _révocation_.

![api-key-revoke](../../../../../static/img/screenshots/api-key-revoke.png)

Une clé révoquée cesse de fonctionner aussitôt, partout où elle est utilisée.
