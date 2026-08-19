# sf-wiki

The StrongFish documentation, as a [Docusaurus](https://docusaurus.io) site.
Published at [doc.strong-fish.com](https://doc.strong-fish.com), and deployed
from the same pipeline as the rest of the repository (see the `wiki` stage in
the root `Dockerfile`).

```shell
npm install
npm start          # http://localhost:3000
npm run build      # both locales
```

## What is here

| Path | What it is |
| --- | --- |
| `docs/` | The English pages. The sidebar is generated from this tree, so a new file is a new page with nothing else to update. |
| `i18n/fr/` | The French ones, plus the navbar/footer strings. Docusaurus mirrors the `docs/` layout under `docusaurus-plugin-content-docs/current/`. |
| `static/img/screenshots/` | Screenshots of the real app, captured against fake data. |

## Translations

French is a full translation, not a fallback: a page that exists in `docs/` and
not in `i18n/fr/` renders in English on the French site. Adding a page means
adding both.

The navbar and footer strings live in `i18n/fr/docusaurus-theme-classic/`;
regenerate the skeletons with `npm run write-translations -- --locale fr` after
changing the navbar.

## Note on the webpack pin

`overrides.webpack` pins 5.105.0. Docusaurus 3.7 passes options to webpack's
ProgressPlugin that 5.106+ rejects outright, so without the pin the build fails
on a schema validation error that has nothing to do with this site.
