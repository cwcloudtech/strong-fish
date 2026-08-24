# AI instruct 24

## Private storage

A bucket or Google drive can be not public: the API has to serve an endpoint checking the profile's visibility and connected user to serve the video/audio file using the stored credentials (and it has to work with video/audio player on web and mobile).

## Storage sharing

A user can share it's own storage in upload with other users in his settings (migrate the storage settings into a new table with associative ACL table, migrate the existing data).

I want owner/reader/writter ACL.
