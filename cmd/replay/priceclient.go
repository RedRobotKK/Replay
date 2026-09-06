package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
)

// The one place in the CLI that reaches the network on the user's behalf, and
// it is behind an explicit flag.
//
// `replay rules --check-prices` answers a question the tool could not answer
// about itself: the compiled price table is dated, and until now the only
// thing anyone could say was how old it was. Age is a prompt to worry. An
// independent source agreeing with it is an answer.
//
// It compares and reports. It does not install, and the engine that does the
// comparing has no network code at all — this fetches bytes and hands them
// over, so the arithmetic stays testable without a socket.
const priceDBURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"

func runCheckPrices(stdout io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, priceDBURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("user-agent", "replay-price-check")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch the price database: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only; a close error tells us nothing actionable
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch the price database: HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return fmt.Errorf("read the price database: %w", err)
	}

	obs, err := cachemodel.ParseLiteLLMPrices(raw)
	if err != nil {
		return err
	}
	res := cachemodel.CheckPrices(obs)

	_, _ = fmt.Fprintf(stdout, "Price table %s, compared against an independent database of %d first-party models.\n\n",
		res.TableVersion, len(obs))

	if len(res.Disagreements) == 0 {
		_, _ = fmt.Fprintf(stdout, "  No disagreement on %d models.\n", len(res.Compared))
	} else {
		_, _ = fmt.Fprintf(stdout, "  %d disagreement(s) across %d compared models:\n\n", len(res.Disagreements), len(res.Compared))
		for _, d := range res.Disagreements {
			_, _ = fmt.Fprintf(stdout, "    %-14s %-7s  ours $%7.2f   theirs $%7.2f   per Mtok   (%s)\n",
				d.Model, d.Field, d.Ours, d.Theirs, d.SourceKey)
		}
	}
	if len(res.Unmatched) > 0 {
		_, _ = fmt.Fprintf(stdout, "\n  Not named by that source, so unchecked rather than confirmed: %v\n", res.Unmatched)
	}
	_, _ = fmt.Fprintf(stdout, "\nThis is a second observer, not an authority. It can be stale or describe a\n"+
		"different SKU under a similar name, so nothing here is installed automatically.\n"+
		"A disagreement is a prompt to check the provider's own page and, if it is\n"+
		"real, to update the table and its date deliberately.\n")
	return nil
}
