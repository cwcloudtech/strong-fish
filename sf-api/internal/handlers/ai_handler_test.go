package handlers

import (
	"testing"

	"strong-fish-api/internal/models"
)

// TestMatchAdapterConnection pins the CWCloud adapter-label match: the ADAPTER
// contains the subscription's name or id (not the other way), with the
// certainty order id-is / name-is / id-contained / longest-name-contained.
func TestMatchAdapterConnection(t *testing.T) {
	const (
		bigID   = "11111111-1111-1111-1111-111111111111"
		drvID   = "22222222-2222-2222-2222-222222222222"
		dailyID = "33333333-3333-3333-3333-333333333333"
		blankID = "44444444-4444-4444-4444-444444444444"
	)
	// Order is the list order ask would see (own before shared, then position);
	// the blank-named row must never match.
	conns := []models.AIConnection{
		{ID: bigID, Name: "Big model"},
		{ID: drvID, Name: "driver"},
		{ID: dailyID, Name: "Daily driver"},
		{ID: blankID, Name: ""},
	}

	cases := []struct {
		name    string
		adapter string
		want    string
	}{
		{"exact name", "Big model", bigID},
		{"exact name, lowercased", "big model", bigID},
		{"exact name, padded", "  BIG MODEL  ", bigID},
		{"exact name, case-insensitive for two-word", "daily driver", dailyID},
		{"the id itself", bigID, bigID},
		{"the id, uppercased", "11111111-1111-1111-1111-111111111111", bigID},
		{"name carried inside a label", "strong-fish big model", bigID},
		{"name carried inside parens", "Sourcing (BIG MODEL)", bigID},
		{"name carried inside a sentence", "ask the big model please", bigID},
		{"id carried inside a label", "sf-" + bigID, bigID},
		{"longest contained name wins", "my daily driver setup", dailyID},
		{"exact short name beats a longer one that contains it", "driver", drvID},
		{"a contained id beats a contained name", "Big model via " + drvID, drvID},
		{"a prefix of a name matches nothing", "big", ""},
		{"the middle of a name matches nothing", "mod", ""},
		{"part of a longer name matches nothing", "Daily dri", ""},
		{"a blank adapter matches nothing", "   ", ""},
		{"a blank-named subscription is never carried by any adapter", "anything at all", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchAdapterConnection(tc.adapter, conns); got != tc.want {
				t.Errorf("matchAdapterConnection(%q) = %q, want %q", tc.adapter, got, tc.want)
			}
		})
	}
}

// TestPickAdapterConnection pins the adapter resolution the CWCloud call uses:
// a match wins; with no match it falls back to the first subscription in order,
// preferring one that has a key saved so an unconfigured first slot does not
// shadow a connected one; and "" only when there are none.
func TestPickAdapterConnection(t *testing.T) {
	const (
		firstID  = "11111111-1111-1111-1111-111111111111"
		secondID = "22222222-2222-2222-2222-222222222222"
	)

	t.Run("a match is spent even when a later one also has a key", func(t *testing.T) {
		conns := []models.AIConnection{
			{ID: firstID, Name: "Big model", HasToken: true},
			{ID: secondID, Name: "driver", HasToken: true},
		}
		if got := pickAdapterConnection("use the driver", conns); got != secondID {
			t.Errorf("got %q, want the matched %q", got, secondID)
		}
	})

	t.Run("no match falls back to the first in order", func(t *testing.T) {
		conns := []models.AIConnection{
			{ID: firstID, Name: "Big model", HasToken: true},
			{ID: secondID, Name: "driver", HasToken: true},
		}
		if got := pickAdapterConnection("nothing here matches", conns); got != firstID {
			t.Errorf("got %q, want the first %q", got, firstID)
		}
	})

	t.Run("no match skips an unconfigured first slot for one with a key", func(t *testing.T) {
		conns := []models.AIConnection{
			{ID: firstID, Name: "empty placeholder", HasToken: false},
			{ID: secondID, Name: "real key", HasToken: true},
		}
		if got := pickAdapterConnection("adapter that matches neither", conns); got != secondID {
			t.Errorf("got %q, want the key-bearing %q", got, secondID)
		}
	})

	t.Run("a blank adapter takes the first key-bearing subscription", func(t *testing.T) {
		conns := []models.AIConnection{
			{ID: firstID, Name: "placeholder", HasToken: false},
			{ID: secondID, Name: "real", HasToken: true},
		}
		if got := pickAdapterConnection("", conns); got != secondID {
			t.Errorf("got %q, want %q", got, secondID)
		}
	})

	t.Run("no key anywhere falls back to the very first", func(t *testing.T) {
		conns := []models.AIConnection{
			{ID: firstID, Name: "a", HasToken: false},
			{ID: secondID, Name: "b", HasToken: false},
		}
		if got := pickAdapterConnection("no match", conns); got != firstID {
			t.Errorf("got %q, want %q", got, firstID)
		}
	})

	t.Run("no subscriptions resolves to empty", func(t *testing.T) {
		if got := pickAdapterConnection("anything", nil); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}
