package stance

import "errors"

// ErrRefused is the provider rejecting a credential the member can fix: a bad
// email or password on pairing. Distinct from a transport failure, which is
// not theirs to act on.
var ErrRefused = errors.New("stance: the provider refused the request")

// ErrReauthorize is the stored grant no longer working - the member revoked it
// at Stance, or it lapsed beyond refreshing. The answer is to pair again.
var ErrReauthorize = errors.New("stance: authorization no longer valid")
