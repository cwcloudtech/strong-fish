# AI instruct 11

## Google drive integration

Like `~/cwclock` I want the service account to be a json file uploaded not a base64 string (even if it remains stored like that in the data model).

## Wiki

Like `~/uprodit` I want a markdown/docusaurus `sf-wiki` folder.
Move the `about.md` inside and add for the frontend an environment variable `SF_ABOUT_URL` which will have as default value `https://doc.strong-fish.com/docs/about`.

The wiki's url will be `https://doc.strong-fish.com`.

I want also tutorials explaining how to signup as athlete or coach (waiting for validation of the superuser), the api key for login with api key in the mobile app (I'll add screenshot later).

Add also an RPE/1RM/e1RM page vocabulary's explanation (RPE8 = RIR2 etc).

Add also how to upload video configuring bucket or google drive.

I also want i18n with French like it's done in `~/cwcloud-website`.

Pick the same logo and update the docker-compose-build.yml and the cicd pipeline to deploy it like it's done in `~/uprodit`.

And same I want light and darkmode.

## Auto-upgrade

Check if the upgrade button like `~/cwclock/cwclock-mobile` is implemented and implement it if it's not the case.
