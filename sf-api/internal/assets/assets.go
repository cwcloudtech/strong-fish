// Package assets embeds strong-fish's static binary assets so they can be
// shared across packages (transactional emails, the public logo endpoint)
// without each one needing its own copy or a go:embed reaching outside its
// directory tree.
package assets

import _ "embed"

//go:embed strongfish-logo.png
var LogoPNG []byte
