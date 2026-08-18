# AI instruct 6

## UX/UI web

* On darkmode I don't want the logo to have a different background color
* The sidebar must be collapsible
* I want like `~/cwclock` a Download mobile app icon (on Desktop it print a QR code in a modal and on Android it's downloading the app)

## Api keys

I want the same api key feature existing in `~/cwclock` with same data model.
I want the mobile authentication to be similar : based on QR code containing:

```
api_url = ...
api_key = ...
```

And same it can also be downloaded as config file for a futur CLI.

## Excel import

I still want to be able to upload program like [program_1](./assets/program_1.xlsx) and even if there's calculation error, the app doesn't take into account the calculations formula and do it itself (it's probably already the case).

I also want to be able to parse program like this one: [program_2](./assets/program_2.xlsx):

Each tabs can be an entire bloc containing multiple weeks (W1 is week 1 for example)

You have to detect both format.
