// Package store is strong-fish's data access layer. Every table is a set of
// indexed columns plus a JSONB `data` payload (see sf-db/V1__init.sql), so each
// store marshals its own payload struct in and out of that column and updates
// it with a shallow merge (`data = data || $n::jsonb`) - never a whole-column
// overwrite, which would silently drop fields the caller didn't know about.
package store

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strconv"

	"strong-fish-api/internal/utils"
)

var (
	// ErrNotFound is returned when a row doesn't exist, or exists but the
	// caller isn't allowed to know it does.
	ErrNotFound = errors.New("not found")
	// ErrDuplicateEmail is returned when a write would collide with another
	// account's email address.
	ErrDuplicateEmail = errors.New("email already in use")
	// ErrDuplicateHandle is returned when a profile handle is already taken.
	ErrDuplicateHandle = errors.New("handle already in use")
	// ErrCannotRemoveOwner is returned when a write would remove or demote a
	// club's owner.
	ErrCannotRemoveOwner = errors.New("the club owner cannot be removed")
	// ErrAlreadyMember is returned when adding a user who is already in the
	// club.
	ErrAlreadyMember = errors.New("this user is already a member of the club")
	// ErrDuplicateExercise is returned when a new exercise's name normalizes
	// onto an existing one.
	ErrDuplicateExercise = errors.New("an exercise with this name already exists")
	// ErrExerciseInUse is returned when deleting an exercise a program still
	// prescribes.
	ErrExerciseInUse = errors.New("this exercise is used by a program")
)

// defaultImagePosition centers a picture when no position was ever stored for
// it (a never-repositioned image, or one saved before the field existed).
const defaultImagePosition = 50.0

func resolveImagePosition(v *float64) float64 {
	if v == nil {
		return defaultImagePosition
	}
	return *v
}

// generateToken mints a URL-safe random secret.
func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return utils.EMPTY, err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// parseFloat reads a numeric value out of a JSONB text extraction (`data->>`),
// which Postgres hands back as a string.
func parseFloat(value string) (float64, error) {
	return strconv.ParseFloat(value, 64)
}

// clampPage normalizes a page/size pair coming from a query string into a
// LIMIT/OFFSET pair, so no caller can ask for a negative offset or an unbounded
// page.
func clampPage(page, size, maxSize int) (limit, offset int) {
	if size <= 0 || size > maxSize {
		size = maxSize
	}
	if page < 0 {
		page = 0
	}
	return size, page * size
}
