package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/utils"
)

const s3Service = "s3"

// s3Target speaks the S3 REST API to any endpoint that implements it - AWS,
// MinIO, Scaleway, DigitalOcean Spaces - with hand-signed SigV4 requests
// rather than an SDK dependency. Path-style addressing (endpoint/bucket/key)
// is used because it works everywhere without a per-bucket DNS name and TLS
// certificate, which a self-hosted MinIO typically doesn't have.
type s3Target struct {
	endpoint      string
	bucket        string
	region        string
	accessKey     string
	secretKey     string
	basePath      string
	publicBaseURL string
	private       bool
	httpClient    *http.Client
}

func newS3Target(conn models.StorageConnection) *s3Target {
	region := conn.Region
	if utils.IsBlank(region) {
		// SigV4 requires *a* region in the credential scope; S3-compatible
		// servers that have no concept of one accept this canonical default.
		region = "us-east-1"
	}
	return &s3Target{
		endpoint:      strings.TrimSuffix(conn.Endpoint, "/"),
		bucket:        conn.BucketName,
		region:        region,
		accessKey:     conn.AccessKey,
		secretKey:     conn.SecretKey,
		basePath:      cleanBasePath(conn.Path),
		publicBaseURL: strings.TrimSuffix(conn.PublicBaseURL, "/"),
		private:       conn.Private,
		httpClient:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (s *s3Target) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	objectKey := key
	if utils.IsNotBlank(s.basePath) {
		objectKey = s.basePath + "/" + key
	}

	requestURL := s.endpoint + "/" + uriEncodePath(s.bucket) + "/" + uriEncodePath(objectKey)
	parsed, err := url.Parse(requestURL)
	if err != nil {
		return utils.EMPTY, fmt.Errorf("storage s3: invalid endpoint: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, requestURL, bytes.NewReader(data))
	if err != nil {
		return utils.EMPTY, err
	}
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = int64(len(data))
	// On a public bucket the object has to be fetchable by a browser with no
	// credentials: the URL goes into a post, and the player is a plain <video>
	// tag. A bucket with ACLs disabled rejects this header, which is the right
	// moment to find out - better a failed upload than a post pointing at an
	// unreadable object.
	//
	// On a private one the header is not merely unnecessary but wrong: a
	// bucket whose policy forbids public objects would refuse the write, which
	// is exactly the case this connection exists for.
	if !s.private {
		req.Header.Set("X-Amz-Acl", "public-read")
	}
	s.sign(req, parsed, data)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return utils.EMPTY, fmt.Errorf("storage s3: upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return utils.EMPTY, fmt.Errorf("storage s3: upload returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if utils.IsNotBlank(s.publicBaseURL) {
		return s.publicBaseURL + "/" + objectKey, nil
	}
	return s.endpoint + "/" + s.bucket + "/" + objectKey, nil
}

// Download reads an object back with the connection's own credentials, for a
// bucket nobody else can read.
//
// The reader's Range header goes through untouched and so does the answer:
// seeking in a video is a partial request, and buffering a 20MB file in this
// app to serve two seconds of it would be both slow and pointless.
func (s *s3Target) Download(ctx context.Context, key, rangeHeader string) (*Object, error) {
	objectKey := key
	if utils.IsNotBlank(s.basePath) {
		objectKey = s.basePath + "/" + key
	}

	requestURL := s.endpoint + "/" + uriEncodePath(s.bucket) + "/" + uriEncodePath(objectKey)
	parsed, err := url.Parse(requestURL)
	if err != nil {
		return nil, fmt.Errorf("storage s3: invalid endpoint: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	if utils.IsNotBlank(rangeHeader) {
		req.Header.Set("Range", rangeHeader)
	}
	s.sign(req, parsed, nil)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("storage s3: download failed: %w", err)
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		resp.Body.Close()
		return nil, fmt.Errorf("storage s3: download returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
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

// sign applies AWS Signature Version 4 to req. Every x-amz-* header set on the
// request is signed, not just a fixed list: the ACL header changes the meaning
// of the request, so leaving it out of the signature would let it be stripped
// or rewritten in flight.
func (s *s3Target) sign(req *http.Request, u *url.URL, body []byte) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	payloadHash := hexSHA256(body)

	req.Header.Set("Host", u.Host)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	names := []string{"host"}
	for name := range req.Header {
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "x-amz-") || lower == "content-type" {
			names = append(names, lower)
		}
	}
	sort.Strings(names)

	var canonicalHeaders strings.Builder
	for _, name := range names {
		canonicalHeaders.WriteString(name)
		canonicalHeaders.WriteByte(':')
		canonicalHeaders.WriteString(strings.TrimSpace(req.Header.Get(name)))
		canonicalHeaders.WriteByte('\n')
	}
	signedHeaders := strings.Join(names, ";")

	canonicalRequest := strings.Join([]string{
		req.Method,
		u.EscapedPath(),
		utils.EMPTY,
		canonicalHeaders.String(),
		signedHeaders,
		payloadHash,
	}, "\n")

	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, s.region, s3Service)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		hexSHA256([]byte(canonicalRequest)),
	}, "\n")

	signature := hex.EncodeToString(hmacSHA256(signingKey(s.secretKey, dateStamp, s.region), stringToSign))
	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.accessKey, credentialScope, signedHeaders, signature,
	))
}

func signingKey(secretKey, dateStamp, region string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, s3Service)
	return hmacSHA256(kService, "aws4_request")
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func hexSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// uriEncodePath percent-encodes a key for both the request URI and the
// canonical request they have to match on byte for byte. Slashes stay literal
// (they are the key's own separators); everything outside the unreserved set
// is encoded, which is stricter than url.PathEscape - it leaves characters
// like '+' alone, and S3 would then hash a different string than it received.
func uriEncodePath(path string) string {
	var b strings.Builder
	for i := 0; i < len(path); i++ {
		c := path[i]
		switch {
		case (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'),
			c == '-', c == '_', c == '.', c == '~', c == '/':
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// cleanBasePath normalizes an optional prefix to "a/b" with no leading or
// trailing slash, so callers can join it with a single "/" either way.
func cleanBasePath(path string) string {
	return strings.Trim(strings.TrimSpace(path), "/")
}
