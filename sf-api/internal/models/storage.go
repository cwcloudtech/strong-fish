package models

import "time"

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
	// Private says a bucket does not allow public objects.
	//
	// It stops the upload asking for a public-read ACL, which such a bucket
	// refuses outright - that refusal is what used to make posting a video
	// impossible on a corporate bucket. It does not change how the object is
	// read: an S3 object is always served through this API, with the stored
	// credentials, to a reader it has checked (see the media handler).
	//
	// It means nothing for a Google Drive target. A Drive file is posted as a
	// Drive link, which requires the file to be shared with anyone holding it;
	// somebody who needs media that is not reachable that way wants a bucket.
	Private bool `json:"private,omitempty"`
}

// What one member may do with one storage.
//
// Three roles rather than a boolean because the two questions are different:
// somebody lending their bucket to their coach wants them to *upload* to it,
// while somebody in a private club wants their club-mates to be able to *play*
// what is in it. Owner is both, plus the right to share and delete it.
const (
	StorageRoleOwner  = "owner"
	StorageRoleWriter = "writer"
	StorageRoleReader = "reader"
)

// IsValidStorageRole reports whether role is one that may be granted.
//
// Owner is excluded: it is set when the storage is created and belongs to
// exactly one account. Handing it out would leave two people able to delete
// the same bucket's connection, and no way to say which of them it is.
func IsValidStorageRole(role string) bool {
	return role == StorageRoleWriter || role == StorageRoleReader
}

// CanWriteStorage reports whether role may upload to the storage.
func CanWriteStorage(role string) bool {
	return role == StorageRoleOwner || role == StorageRoleWriter
}

// CanReadStorage reports whether role may read objects out of it. Every role
// can: a grant of any kind is somebody saying "you may see what is in here".
func CanReadStorage(role string) bool {
	return role == StorageRoleOwner || role == StorageRoleWriter || role == StorageRoleReader
}

// Storage is one connection as it is stored: the credentials, who owns it, and
// - when it was read for somebody in particular - what that person may do with
// it.
type Storage struct {
	ID      string            `json:"id"`
	OwnerID string            `json:"ownerId"`
	Name    string            `json:"name,omitempty"`
	Conn    StorageConnection `json:"connection"`
	// Position is where this target sits in its owner's priority order: an
	// upload goes to every one of them, and the link in the post comes from
	// the first.
	Position int `json:"position"`
	// Role is the caller's own role, filled in by the queries that read a
	// storage for somebody. Empty when it was read without a caller in mind.
	Role      string    `json:"role,omitempty"`
	OwnerName string    `json:"ownerName,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// StorageGrant is one line of a storage's access list, as its owner reads it.
type StorageGrant struct {
	UserID    string    `json:"userId"`
	Name      string    `json:"name"`
	Handle    string    `json:"handle,omitempty"`
	Picture   string    `json:"picture,omitempty"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
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
