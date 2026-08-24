package storage

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/utils"
)

const (
	driveUploadAPIBase = "https://www.googleapis.com/upload/drive/v3/files"
	driveAPIBase       = "https://www.googleapis.com/drive/v3/files"
	driveScope         = "https://www.googleapis.com/auth/drive"
	driveFolderMime    = "application/vnd.google-apps.folder"
	defaultTokenURI    = "https://oauth2.googleapis.com/token"
)

// serviceAccountKey is the part of a Google service-account JSON key this
// package needs.
type serviceAccountKey struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

// DecodeServiceAccount validates a base64 service-account key and returns the
// account's email. It is exported so a connection can be rejected at save time
// rather than silently failing on the first upload, weeks later.
func DecodeServiceAccount(base64JSON string) (string, error) {
	key, _, err := parseServiceAccount(base64JSON)
	if err != nil {
		return utils.EMPTY, err
	}
	return key.ClientEmail, nil
}

func parseServiceAccount(base64JSON string) (serviceAccountKey, *rsa.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(base64JSON))
	if err != nil {
		return serviceAccountKey{}, nil, fmt.Errorf("invalid base64: %w", err)
	}
	var key serviceAccountKey
	if err := json.Unmarshal(raw, &key); err != nil {
		return serviceAccountKey{}, nil, fmt.Errorf("invalid service account JSON: %w", err)
	}
	if utils.IsBlank(key.ClientEmail) || utils.IsBlank(key.PrivateKey) {
		return serviceAccountKey{}, nil, fmt.Errorf("service account JSON is missing client_email or private_key")
	}
	if utils.IsBlank(key.TokenURI) {
		key.TokenURI = defaultTokenURI
	}

	block, _ := pem.Decode([]byte(key.PrivateKey))
	if block == nil {
		return serviceAccountKey{}, nil, fmt.Errorf("service account private_key is not valid PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return serviceAccountKey{}, nil, fmt.Errorf("could not parse the service account private key: %w", err)
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return serviceAccountKey{}, nil, fmt.Errorf("the service account private key is not an RSA key")
	}
	return key, rsaKey, nil
}

// driveTarget talks to the Drive v3 REST API directly, authenticating as a
// service account with a hand-signed JWT assertion - no google-api-go-client
// dependency for what amounts to three HTTP calls.
type driveTarget struct {
	key        serviceAccountKey
	privateKey *rsa.PrivateKey
	folderID   string
	basePath   string
	private    bool
	httpClient *http.Client
}

func newDriveTarget(conn models.StorageConnection) (*driveTarget, error) {
	key, privateKey, err := parseServiceAccount(conn.ServiceAccountBase64)
	if err != nil {
		return nil, fmt.Errorf("storage google_drive: %w", err)
	}
	return &driveTarget{
		key:        key,
		privateKey: privateKey,
		folderID:   conn.FolderID,
		basePath:   cleanBasePath(conn.Path),
		private:    conn.Private,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}, nil
}

func (d *driveTarget) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	token, err := d.accessToken(ctx)
	if err != nil {
		return utils.EMPTY, err
	}

	// Drive addresses files by id and has no notion of a key with slashes in
	// it, so the key's last segment becomes the file's name.
	name := key
	if index := strings.LastIndex(name, "/"); index >= 0 {
		name = name[index+1:]
	}

	// Drive addresses folders by id, not by a path string, so a subfolder has
	// to be walked one level at a time - and created where it does not exist
	// yet, since a member typing "strong-fish/videos" means "put them there",
	// not "fail unless I made those folders by hand first".
	parent, err := d.ensureBaseFolder(ctx, token)
	if err != nil {
		return utils.EMPTY, err
	}

	fileID, err := d.createFile(ctx, token, name, parent, data, contentType)
	if err != nil {
		return utils.EMPTY, err
	}
	// On a private folder nothing is granted: the file stays visible to the
	// service account alone, and the API reads it back through Download for
	// readers it has checked. The id is what identifies it from then on.
	if d.private {
		return fileID, nil
	}

	// A file in a service account's own folder is invisible to everybody else,
	// including the person about to read the post. Granting anyone-with-the-
	// link reader access is what makes the returned URL work at all.
	if err := d.shareWithAnyone(ctx, token, fileID); err != nil {
		return utils.EMPTY, err
	}

	// Drive's /preview endpoint is an embeddable player; its direct-download
	// URL serves an interstitial for files this size, which a <video> tag
	// cannot get past. media-player recognises this shape and frames it.
	return "https://drive.google.com/file/d/" + fileID + "/preview", nil
}

// Download reads a file back with the service account's own credentials, for a
// folder nobody else can see into.
//
// key is the file id here rather than a path: Drive has no keys, and it is the
// id that Upload returned and that the media URL carries. alt=media is what
// asks for the bytes rather than the metadata.
func (d *driveTarget) Download(ctx context.Context, key, rangeHeader string) (*Object, error) {
	token, err := d.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		driveAPIBase+"/"+url.PathEscape(key)+"?alt=media&supportsAllDrives=true", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if utils.IsNotBlank(rangeHeader) {
		req.Header.Set("Range", rangeHeader)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("storage google_drive: download failed: %w", err)
	}
	if resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		resp.Body.Close()
		return nil, fmt.Errorf("storage google_drive: download returned %d: %s",
			resp.StatusCode, strings.TrimSpace(string(message)))
	}

	return &Object{
		Body:          resp.Body,
		ContentType:   resp.Header.Get("Content-Type"),
		ContentLength: resp.ContentLength,
		ContentRange:  resp.Header.Get("Content-Range"),
		AcceptRanges:  resp.Header.Get("Accept-Ranges"),
		StatusCode:    resp.StatusCode,
	}, nil
}

func (d *driveTarget) accessToken(ctx context.Context) (string, error) {
	now := time.Now().UTC()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{
		"iss":   d.key.ClientEmail,
		"scope": driveScope,
		"aud":   d.key.TokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})
	signingInput := base64URLEncode(header) + "." + base64URLEncode(claims)

	hashed := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, d.privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return utils.EMPTY, fmt.Errorf("storage google_drive: could not sign the JWT: %w", err)
	}

	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {signingInput + "." + base64URLEncode(signature)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.key.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return utils.EMPTY, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return utils.EMPTY, fmt.Errorf("storage google_drive: token request failed: %w", err)
	}
	defer resp.Body.Close()

	var body struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return utils.EMPTY, fmt.Errorf("storage google_drive: invalid token response: %w", err)
	}
	if utils.IsBlank(body.AccessToken) {
		return utils.EMPTY, fmt.Errorf("storage google_drive: token request rejected: %s", body.Error)
	}
	return body.AccessToken, nil
}

// ensureBaseFolder walks the configured subfolder down from the root folder,
// creating whichever level is missing, and returns the folder uploads go into.
// An empty path is simply the root.
func (d *driveTarget) ensureBaseFolder(ctx context.Context, token string) (string, error) {
	folder := d.folderID
	if utils.IsBlank(d.basePath) {
		return folder, nil
	}

	for _, segment := range strings.Split(d.basePath, "/") {
		next, err := d.ensureFolder(ctx, token, segment, folder)
		if err != nil {
			return utils.EMPTY, err
		}
		folder = next
	}
	return folder, nil
}

func (d *driveTarget) ensureFolder(ctx context.Context, token, name, parentID string) (string, error) {
	query := fmt.Sprintf(
		"name = '%s' and '%s' in parents and mimeType = '%s' and trashed = false",
		escapeDriveQuery(name), escapeDriveQuery(parentID), driveFolderMime,
	)

	id, found, err := d.searchOne(ctx, token, query)
	if err != nil {
		return utils.EMPTY, err
	}
	if found {
		return id, nil
	}
	return d.createFolder(ctx, token, name, parentID)
}

func (d *driveTarget) searchOne(ctx context.Context, token, query string) (string, bool, error) {
	params := url.Values{
		"q":                         {query},
		"fields":                    {"files(id)"},
		"supportsAllDrives":         {"true"},
		"includeItemsFromAllDrives": {"true"},
		"spaces":                    {"drive"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, driveAPIBase+"?"+params.Encode(), nil)
	if err != nil {
		return utils.EMPTY, false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return utils.EMPTY, false, fmt.Errorf("storage google_drive: search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return utils.EMPTY, false, fmt.Errorf("storage google_drive: search returned %d: %s",
			resp.StatusCode, strings.TrimSpace(string(message)))
	}

	var list struct {
		Files []struct {
			ID string `json:"id"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return utils.EMPTY, false, fmt.Errorf("storage google_drive: invalid search response: %w", err)
	}
	if len(list.Files) == 0 {
		return utils.EMPTY, false, nil
	}
	return list.Files[0].ID, true, nil
}

func (d *driveTarget) createFolder(ctx context.Context, token, name, parentID string) (string, error) {
	payload, _ := json.Marshal(map[string]any{
		"name":     name,
		"mimeType": driveFolderMime,
		"parents":  []string{parentID},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		driveAPIBase+"?fields=id&supportsAllDrives=true", bytes.NewReader(payload))
	if err != nil {
		return utils.EMPTY, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return utils.EMPTY, fmt.Errorf("storage google_drive: creating a folder failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return utils.EMPTY, fmt.Errorf("storage google_drive: creating a folder returned %d: %s",
			resp.StatusCode, strings.TrimSpace(string(message)))
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil || utils.IsBlank(created.ID) {
		return utils.EMPTY, fmt.Errorf("storage google_drive: creating a folder returned no id")
	}
	return created.ID, nil
}

// escapeDriveQuery quotes a value for Drive's query language, where a bare
// apostrophe in a folder name would otherwise end the string and change what
// the query means.
func escapeDriveQuery(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `'`, `\'`)
}

func (d *driveTarget) createFile(ctx context.Context, token, name, parentID string, data []byte, contentType string) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	metadata, _ := json.Marshal(map[string]any{"name": name, "parents": []string{parentID}})
	metaPart, err := writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json; charset=UTF-8"}})
	if err != nil {
		return utils.EMPTY, err
	}
	if _, err := metaPart.Write(metadata); err != nil {
		return utils.EMPTY, err
	}

	mediaPart, err := writer.CreatePart(textproto.MIMEHeader{"Content-Type": {contentType}})
	if err != nil {
		return utils.EMPTY, err
	}
	if _, err := mediaPart.Write(data); err != nil {
		return utils.EMPTY, err
	}
	if err := writer.Close(); err != nil {
		return utils.EMPTY, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		driveUploadAPIBase+"?uploadType=multipart&supportsAllDrives=true&fields=id", body)
	if err != nil {
		return utils.EMPTY, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "multipart/related; boundary="+writer.Boundary())

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return utils.EMPTY, fmt.Errorf("storage google_drive: upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return utils.EMPTY, fmt.Errorf("storage google_drive: upload returned %d: %s", resp.StatusCode, strings.TrimSpace(string(message)))
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil || utils.IsBlank(created.ID) {
		return utils.EMPTY, fmt.Errorf("storage google_drive: upload returned no file id")
	}
	return created.ID, nil
}

func (d *driveTarget) shareWithAnyone(ctx context.Context, token, fileID string) error {
	payload, _ := json.Marshal(map[string]string{"role": "reader", "type": "anyone"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		driveAPIBase+"/"+fileID+"/permissions?supportsAllDrives=true", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("storage google_drive: sharing failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("storage google_drive: sharing returned %d: %s", resp.StatusCode, strings.TrimSpace(string(message)))
	}
	return nil
}

func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}
