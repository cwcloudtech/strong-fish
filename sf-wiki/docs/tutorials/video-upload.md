---
id: video-upload
title: Posting a video
sidebar_position: 3
---

You can attach a video to a post - a competition attempt, a form check - but it
goes into **your own storage**, not ours.

## Why your own bucket

strong-fish hosts no video at all. Twenty megabytes per post would wreck the
database, and paying to serve other people's training footage is not what this
app is for.

So you bring a destination: an **S3-compatible bucket** or a **Google Drive
folder**. The app uploads there and the post carries a link to the file. Until
you configure one, the video button answers *"set up your storage first"* - the
API returns a 405, which is the app saying the request was fine, the feature is
simply not available on your account yet.

## Option 1: an S3-compatible bucket

Works with AWS S3, MinIO, Scaleway, DigitalOcean Spaces - anything speaking the
S3 API.

1. Create a bucket, and an access key that can write to it.
2. **Object reads must be public.** The link goes into a post and is played by a
   plain video player in someone else's browser, with no credentials. strong-fish
   uploads each object with a public-read ACL; a bucket with ACLs disabled will
   reject the upload, which is the right moment to find out.
3. In the app: **Settings → Video storage**, pick *S3-compatible bucket*.
4. Fill in:

   | Field | Example |
   | --- | --- |
   | Endpoint | `https://s3.eu-west-3.amazonaws.com` |
   | Region | `eu-west-3` |
   | Bucket | `my-lifting-videos` |
   | Access key / Secret key | from your provider |
   | Subfolder *(optional)* | `strong-fish` |
   | Public address *(optional)* | your CDN or custom domain, if the bucket is served through one |

5. Save.

![Video storage configured against an S3 bucket](/img/screenshots/video-storage.png)

## Option 2: a Google Drive folder

1. In the Google Cloud console, create a **service account** and download its
   **JSON key**.
2. Create (or pick) a Drive folder, and **share it with the service account's
   email address** with edit rights. Without this it cannot write anything -
   this is the step people miss.
3. Copy the folder's id: it is the last part of its URL,
   `https://drive.google.com/drive/folders/<this bit>`.
4. In the app: **Settings → Video storage**, pick *Google Drive folder*.
5. Upload the JSON key file and paste the folder id.
6. Save.

strong-fish grants each uploaded file anyone-with-the-link read access as it
writes it, and posts the Drive preview player.

## Posting

1. Go to the feed and start a post.
2. Choose **Add a video** and pick a file - MP4, WebM or MOV, up to 20MB by
   default.
3. The upload's URL is appended to your post's text.
4. Write whatever you want around it and post.

The player appears automatically. There is no separate link field: **the first
URL in a post is its embed**, whether you uploaded it or pasted a YouTube link.

## Troubleshooting

| What you see | What it usually is |
| --- | --- |
| "Set up your own video storage" | No storage configured yet. |
| "Your storage rejected this upload" | Wrong keys, wrong bucket name, or ACLs disabled on the bucket. |
| "This video is too large" | Over the size limit - the app tells you what it is on the storage screen. |
| "Not a video a browser can play" | Re-encode as MP4 (H.264). |
| The post shows a link card, not a player | The file's URL is not publicly readable. Check the bucket's policy, or the Drive folder's sharing. |
