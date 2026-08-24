# AI instruct 26

## Upload

Upload still not work with mobile and Google drive.

I'm expecting a link in the post after the upload (it's working on web)

Here's the rules:
* if it's on S3, I want the api to proxify the S3 buckets using the stored credentials and serve the video/audio compliant with the media player component.
* if it's on Google Drive and if Google Drive video/audio files cannot be proxified in read mode for a web/mobile player, post a google drive link like it's already implemented.

I want also to be able to have like `~/cwclock` multiple storage targets (multiple S3 or Google drive folders) with a priority order:
* upload on every target
* read link on the first target (with order number)

Migrate the existing data if you have to change the data model.
