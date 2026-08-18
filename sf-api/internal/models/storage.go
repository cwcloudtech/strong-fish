package models

// Storage connection types. The shape is cwclock's external connections, cut
// down to the two destinations a member would actually keep their own videos
// in - there is no git target here, because a repository is a bad place for
// 20MB of video.
const (
	StorageTypeS3          = "s3"
	StorageTypeGoogleDrive = "google_drive"
)

// StorageConnection is where one member's uploaded videos go.
//
// It belongs to the member, not to the app: strong-fish never hosts video
// itself, so somebody who wants to post one brings their own bucket. Only the
// fields relevant to Type are populated - S3 uses
// Endpoint/BucketName/Region/AccessKey/SecretKey, Google Drive uses
// ServiceAccountBase64/FolderID.
type StorageConnection struct {
	Type       string `json:"type"`
	Endpoint   string `json:"endpoint,omitempty"`
	BucketName string `json:"bucketName,omitempty"`
	Region     string `json:"region,omitempty"`
	AccessKey  string `json:"accessKey,omitempty"`
	// SecretKey and ServiceAccountBase64 are credentials: they are accepted on
	// write and never sent back (see StorageConnection.Redacted), so a
	// compromised session can't read out the keys to somebody's bucket.
	SecretKey            string `json:"secretKey,omitempty"`
	ServiceAccountBase64 string `json:"serviceAccountBase64,omitempty"`
	FolderID             string `json:"folderId,omitempty"`
	// Path is an optional prefix inside the bucket or folder. Empty means its
	// root.
	Path string `json:"path,omitempty"`
	// PublicBaseURL overrides the URL a stored object is addressed by, for a
	// bucket served through a CDN or a custom domain. Empty means the object
	// is addressed at the endpoint it was written to.
	PublicBaseURL string `json:"publicBaseUrl,omitempty"`
}

// Configured reports whether this connection has enough to attempt an upload.
// It is deliberately a shape check, not a reachability one - the first upload
// is what finds out whether the credentials actually work.
func (c StorageConnection) Configured() bool {
	switch c.Type {
	case StorageTypeS3:
		return c.Endpoint != "" && c.BucketName != "" && c.AccessKey != "" && c.SecretKey != ""
	case StorageTypeGoogleDrive:
		return c.ServiceAccountBase64 != "" && c.FolderID != ""
	}
	return false
}

// Redacted is the connection as the owner may read it back: everything needed
// to see what is configured, with the secrets replaced by a marker so the UI
// can show "a key is set" without ever holding the key.
func (c StorageConnection) Redacted() StorageConnection {
	redacted := c
	if redacted.SecretKey != "" {
		redacted.SecretKey = SecretSet
	}
	if redacted.ServiceAccountBase64 != "" {
		redacted.ServiceAccountBase64 = SecretSet
	}
	return redacted
}

// SecretSet is what a configured-but-unreadable credential reads as. A client
// echoing it back on save means "leave the stored secret alone", which is what
// lets someone change their bucket name without retyping their secret key.
const SecretSet = "__set__"
