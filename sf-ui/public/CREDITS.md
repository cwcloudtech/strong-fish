# Image credits

## auth-background.jpg

The photograph behind the login and sign-up screens: a deadlift at an IPF
(International Powerlifting Federation) European Championships.

* **Title:** Alessio Pavone deadlift European Championship 2022
* **Source:** [Wikimedia Commons](https://commons.wikimedia.org/wiki/File:Alessio_Pavone_deadlift_European_Championship2022.jpg)
* **Author:** FactNoter ([White Lights Media](https://whitelightsmedia.com/))
* **Licence:** [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/)
* **Changes:** downscaled from 3277×4096 to 1400×1750 and re-encoded as a
  progressive JPEG. The frame is not cropped — the auth screen crops it for
  display with CSS `object-fit`, which does not alter this file.

### What the licence requires

CC BY-SA 4.0 is a free licence, but unlike public domain it carries two
obligations:

* **Attribution.** The auth screen displays the author and links both the
  source and the licence (see `AuthLayout.jsx`), so the credit travels with the
  image wherever the app is deployed. Removing that credit would breach the
  licence.
* **ShareAlike.** If you *adapt* the photograph — crop, retouch, composite it
  into something else — the adaptation has to be released under CC BY-SA 4.0
  too.

**This does not affect the rest of the project.** ShareAlike attaches to the
image and to adaptations of it, not to the software that displays it;
strong-fish's own code stays MIT. Swapping this file for a public-domain or
CC0 photograph removes the obligation entirely — update the credit in
`AuthLayout.jsx` and the `auth.photoCreditBy` string if you do.

## Logo

`logo*.png`, `favicon*.ico` and the mobile launcher icons are derived from
[`ai-gen/assets/logo.png`](../../ai-gen/assets/logo.png), the project's own
artwork. The `-dark` variants are re-inked for dark backgrounds (see the
project README's "Design" section).
