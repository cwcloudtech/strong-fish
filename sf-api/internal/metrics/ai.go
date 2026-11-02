package metrics

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// AISpend is one answer's cost, as the thing that paid for it.
//
// Four dimensions because four questions get asked of this: what did this
// member spend, what did this key spend (a coach lending one to a roster
// wants the total, not a column per athlete), which provider was it spent
// with, and on which model.
type AISpend struct {
	// UserID is who asked.
	UserID string
	// ConnectionID is the subscription it was spent on, and OwnerID whose it
	// is - the two differ exactly when somebody is using a lent key, which is
	// the case worth being able to total.
	ConnectionID string
	OwnerID      string
	Provider     string
	Model        string
	// Feature says what the tokens went on - the chat, a session reading, a
	// health reading - so a bill can be read as "the chat costs this much"
	// rather than only as a sum.
	Feature string

	Input  int64
	Output int64
	Cached int64
}

// The counter, created the first time something is recorded.
//
// Lazily rather than in Setup: the handlers that spend tokens are built before
// the meter provider is, and a counter captured at init would be bound to the
// no-op provider for the life of the process. Reading the global provider at
// first use means a deployment with metrics switched off simply records into
// the no-op, which is what it is for.
var (
	aiTokensOnce sync.Once
	aiTokens     metric.Int64Counter
)

func aiTokensCounter() metric.Int64Counter {
	aiTokensOnce.Do(func() {
		counter, err := otel.Meter(meterName).Int64Counter("strong_fish_ai_tokens_total",
			metric.WithDescription("Tokens spent on the AI assistant, per member, subscription, provider and model"),
			metric.WithUnit("{token}"))
		if err != nil {
			// A counter that could not be created is one that records
			// nothing; it must not take an answer down with it.
			return
		}
		aiTokens = counter
	})
	return aiTokens
}

// RecordAISpend counts one answer's tokens.
//
// Three series per answer rather than three counters: input, output and
// cached are the same measurement under a label, which is how a dashboard
// adds them up or splits them without knowing three metric names. A kind that
// came back as zero is not recorded at all - a provider that does not report
// cached input should leave no cached series behind.
func RecordAISpend(ctx context.Context, spend AISpend) {
	counter := aiTokensCounter()
	if counter == nil {
		return
	}

	base := []attribute.KeyValue{
		attribute.String("user", spend.UserID),
		attribute.String("subscription", spend.ConnectionID),
		attribute.String("owner", spend.OwnerID),
		attribute.String("provider", spend.Provider),
		attribute.String("model", spend.Model),
		attribute.String("feature", spend.Feature),
	}

	for kind, count := range map[string]int64{
		"input":  spend.Input,
		"output": spend.Output,
		"cached": spend.Cached,
	} {
		if count <= 0 {
			continue
		}
		counter.Add(ctx, count, metric.WithAttributes(
			append(append([]attribute.KeyValue{}, base...), attribute.String("kind", kind))...))
	}
}
