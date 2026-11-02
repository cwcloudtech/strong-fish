package ai

import "testing"

// TestUsageOfReadsEveryProvidersVocabulary covers the three names the same
// three counts arrive under. Getting one wrong does not fail anything - it
// silently bills a member zero tokens, which is the kind of bug a dashboard
// is trusted to have ruled out.
func TestUsageOfReadsEveryProvidersVocabulary(t *testing.T) {
	cases := map[string]struct {
		usage map[string]any
		want  Usage
	}{
		"openai": {
			usage: map[string]any{
				"prompt_tokens":         float64(1200),
				"completion_tokens":     float64(340),
				"prompt_tokens_details": map[string]any{"cached_tokens": float64(1024)},
			},
			want: Usage{Input: 1200, Output: 340, Cached: 1024},
		},
		"anthropic": {
			usage: map[string]any{
				"input_tokens":                float64(1200),
				"output_tokens":               float64(340),
				"cache_read_input_tokens":     float64(900),
				"cache_creation_input_tokens": float64(124),
			},
			want: Usage{Input: 1200, Output: 340, Cached: 1024},
		},
		"gemini": {
			usage: map[string]any{
				"promptTokenCount":        float64(1200),
				"candidatesTokenCount":    float64(340),
				"cachedContentTokenCount": float64(1024),
			},
			want: Usage{Input: 1200, Output: 340, Cached: 1024},
		},
		// A gateway that answers in strings is still counted; one that
		// reports nothing bills nothing rather than failing the answer.
		"a gateway sending strings": {
			usage: map[string]any{"prompt_tokens": "12", "completion_tokens": "7"},
			want:  Usage{Input: 12, Output: 7},
		},
		"nothing reported": {usage: nil, want: Usage{}},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			if got := UsageOf(test.usage); got != test.want {
				t.Errorf("read %+v, want %+v", got, test.want)
			}
		})
	}
}
