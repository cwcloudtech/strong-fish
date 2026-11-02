// Package templates embeds strong-fish's shared Go templates (transactional
// emails) so they live in their own reviewable files instead of Go string
// literals, and so they can be shared across packages without a go:embed
// reaching outside its own directory tree.
package templates

import _ "embed"

//go:embed email.tpl.html
var EmailHTML string

// The chat notification, one markdown file per language.
//
// Markdown rather than HTML, and files rather than Go literals, because these
// are the strings most likely to be rewritten by somebody who is not changing
// code - a translator, or whoever decides the wording is too curt. The first
// `# ` line is the subject (see markdown.Split), so a reworded email and its
// subject line cannot drift apart.
//
//go:embed message.en.md
var messageEmailEN string

//go:embed message.fr.md
var messageEmailFR string

//go:embed message.ar.md
var messageEmailAR string

// The model's instructions, as markdown skills.
//
// Files rather than Go string literals because these are meant to be tuned:
// how a coach should answer is a judgement that gets revised, and revising it
// should not mean reading Go. The feedback skill builds on the coaching one -
// both are sent, the second after the first - so what is common lives in one
// place and only what is specific to a session review is repeated.
//
//go:embed coach.skill.md
var CoachSkill string

//go:embed feedback.skill.md
var FeedbackSkill string

//go:embed health.skill.md
var HealthSkill string

// MessageEmail is the template for a locale, falling back to English for a
// language nothing has been translated into yet - an email in the wrong
// language beats no email at all.
func MessageEmail(locale string) string {
	switch locale {
	case "fr":
		return messageEmailFR
	case "ar":
		return messageEmailAR
	}
	return messageEmailEN
}
