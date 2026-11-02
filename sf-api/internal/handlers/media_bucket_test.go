package handlers

import (
	"testing"

	"strong-fish-api/internal/models"
)

// TestSameBucketMatchesTheSamePlaceWrittenDifferently covers the question a
// media link asks after its storage was deleted and added again: is this new
// connection the same bucket the video is actually in?
//
// Getting it wrong is silent either way - too strict and the video stays
// broken, too loose and the API goes looking for somebody's object in a
// bucket that has nothing to do with it - so the forms an endpoint is written
// in are pinned here.
func TestSameBucketMatchesTheSamePlaceWrittenDifferently(t *testing.T) {
	s3 := func(endpoint, bucket string) models.StorageConnection {
		return models.StorageConnection{
			Type: models.StorageTypeS3, Endpoint: endpoint, BucketName: bucket,
		}
	}

	same := []struct {
		name string
		a, b models.StorageConnection
	}{
		{"identical", s3("https://s3.fr-par.scw.cloud", "strong-fish"),
			s3("https://s3.fr-par.scw.cloud", "strong-fish")},
		{"with and without a scheme", s3("https://s3.example.com", "videos"),
			s3("s3.example.com", "videos")},
		{"trailing slash and case", s3("https://S3.Example.com/", "videos"),
			s3("https://s3.example.com", "videos")},
		{"virtual-hosted against path style", s3("https://videos.s3.example.com", "videos"),
			s3("https://s3.example.com", "videos")},
	}
	for _, test := range same {
		t.Run(test.name, func(t *testing.T) {
			if !sameBucket(test.a, test.b) {
				t.Errorf("%q and %q should be the same bucket", test.a.Endpoint, test.b.Endpoint)
			}
		})
	}

	different := []struct {
		name string
		a, b models.StorageConnection
	}{
		{"another bucket", s3("https://s3.example.com", "videos"),
			s3("https://s3.example.com", "backups")},
		{"another provider", s3("https://s3.fr-par.scw.cloud", "videos"),
			s3("https://s3.eu-west-1.amazonaws.com", "videos")},
		{"another port", s3("http://minio.local:9000", "videos"),
			s3("http://minio.local:9001", "videos")},
	}
	for _, test := range different {
		t.Run(test.name, func(t *testing.T) {
			if sameBucket(test.a, test.b) {
				t.Errorf("%q and %q are not the same bucket", test.a.Endpoint, test.b.Endpoint)
			}
		})
	}
}
