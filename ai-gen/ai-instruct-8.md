# AI instruct 8

## Newspaper/social network

* I don't want to separate the link from post: the first url has to be detected as a link or video link automatically
* I want also the user to be able to upload video with a max size of 20mb configurable with an environment variable in it's own object storage bucket or google drive (user can configure a bucket like it's implemented for organization in `~/cwclock`). If no bucket configured, toast a 405 error from the API when he try to upload. If it's configured, the post put a link to the uploaded file and try to load it in an embed player using the video player webcomponent.
* I want a calendar of event compliant with outlook or google calendar like it's implemented in `~/cwclock` for competition or kind of event
