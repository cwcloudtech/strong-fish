package store

import "fmt"

// A member can choose to be known by their username alone. That choice has to
// hold everywhere their name surfaces - the author of a post, the sender of a
// message, a club's member list, a search result - and those names are built in
// SQL, in a dozen different queries.
//
// So the rule is written once, here, as an expression the queries interpolate,
// rather than as a Go check each of them has to remember to apply. Forgetting
// one would not fail a build or a test; it would quietly print somebody's real
// name next to a post they anonymized.
//
// The username is the fallback's fallback: an account that turned anonymity on
// without setting a username still has a handle, and showing that is better
// than showing an empty string.

// displayName is the SQL for the first name shown for the user at alias.
func displayName(alias string) string {
	return fmt.Sprintf(
		`CASE WHEN %[1]s.data->>'anonymous' = 'true'
		      THEN coalesce(nullif(%[1]s.data->>'username', ''), %[1]s.data->>'handle', '')
		      ELSE coalesce(%[1]s.data->>'name', '') END`, alias)
}

// displaySurname is the surname shown for the user at alias. An anonymized
// member has no surname to show: the username stands alone, and repeating it
// in both fields would render as "marie marie".
func displaySurname(alias string) string {
	return fmt.Sprintf(
		`CASE WHEN %[1]s.data->>'anonymous' = 'true' THEN ''
		      ELSE coalesce(%[1]s.data->>'surname', '') END`, alias)
}

// displayFullName is the two joined, for the queries that carry one string
// rather than a pair. Trimmed, because an anonymized member contributes no
// surname and would otherwise arrive with a trailing space.
func displayFullName(alias string) string {
	return fmt.Sprintf(`trim(%s || ' ' || %s)`, displayName(alias), displaySurname(alias))
}
