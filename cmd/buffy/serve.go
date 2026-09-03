package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/RedRobotKK/Buffy/internal/analysis"
	"github.com/RedRobotKK/Buffy/internal/ledger"
	"github.com/RedRobotKK/Buffy/internal/policy"
	"github.com/RedRobotKK/Buffy/internal/proxy"
)

// Defaults for serve. The listen port is the one the README documents.
const (
	defaultListen          = "127.0.0.1:4000"
	defaultUpstream        = "https://api.anthropic.com"
	ledgerDirName          = "ledger"
	defaultBreakerCooldown = 30 * time.Second
)

// Environment variables that control serve without flags.
const (
	envDisabled = "BUFFY_DISABLED"
	envToken    = "BUFFY_TOKEN"
	envUpstream = "BUFFY_UPSTREAM"
	// envNoPolicy turns every live policy off without touching flags, so
	// a user can rule Buffy's edits out in one move while keeping the
	// ledger and the guards.
	envNoPolicy = "BUFFY_NO_POLICY"
)

// errDisabled is returned when the kill switch is set.
var errDisabled = errors.New(envDisabled + " is set; Buffy will not start. To bypass Buffy entirely, unset ANTHROPIC_BASE_URL")

func runServe(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	listen := fs.String("listen", defaultListen, "loopback address to bind")
	upstream := fs.String("upstream", envOr(envUpstream, defaultUpstream), "provider base URL")
	ledgerDir := fs.String("ledger", "", "ledger directory (default ~/.buffy/ledger)")
	token := fs.String("token", "", "require this value in the "+proxy.HeaderToken+" header (or set "+envToken+")")
	maxSession := fs.Int("max-session-tokens", 0, "refuse a session's next request once it has consumed this many tokens (0 = off)")
	maxDay := fs.Int("max-day-tokens", 0, "refuse requests once this many tokens were consumed today, UTC (0 = off)")
	loopWarn := fs.Int("loop-warn", 0, "add a warning header when one identical tool call repeats this many times (0 = off)")
	loopBlock := fs.Int("loop-block", 0, "refuse the request when one identical tool call repeats this many times (0 = off)")
	breakerFailures := fs.Int("breaker-failures", 0, "open the circuit after this many consecutive provider failures (0 = off)")
	breakerCooldown := fs.Duration("breaker-cooldown", defaultBreakerCooldown, "how long the circuit stays open")
	retries := fs.Int("retries", 0, "resend a request up to this many times on rate limit, overload, server error, or connection failure, before any response byte reached the client (0 = off)")
	retryBase := fs.Duration("retry-base", proxy.DefaultRetryBaseDelay, "first retry backoff; doubles with jitter, capped by -retry-max; a provider Retry-After within the cap replaces it")
	retryMax := fs.Duration("retry-max", proxy.DefaultRetryMaxDelay, "longest wait before a retry")
	editTrigger := fs.Int("context-edit-trigger", 0, "EXPERIMENTAL: ask the provider to clear old tool results once the prompt passes this many tokens, on requests whose client enabled "+policy.BetaFeature+" and set no context_management of its own (0 = off; "+envNoPolicy+"=1 forces off)")
	editKeep := fs.Int("context-edit-keep", analysis.ContextEditKeepLast, "how many recent tool results a clear keeps")
	if err := fs.Parse(args); err != nil {
		return errUsage
	}
	contextEdit, err := contextEditFromFlags(*editTrigger, *editKeep, os.Getenv(envNoPolicy) != "")
	if err != nil {
		return err
	}
	if os.Getenv(envDisabled) != "" {
		return errDisabled
	}
	// The environment value is applied after parsing so it never appears
	// as a default in the usage text.
	if *token == "" {
		*token = os.Getenv(envToken)
	}
	target, err := url.Parse(*upstream)
	if err != nil || target.Scheme == "" || target.Host == "" {
		return fmt.Errorf("upstream must be an absolute URL, got %q", *upstream)
	}
	dir := *ledgerDir
	if dir == "" {
		dir, err = defaultLedgerDir()
		if err != nil {
			return err
		}
	}
	store, err := ledger.Open(dir)
	if err != nil {
		return err
	}
	srv, err := proxy.New(proxy.Config{
		Listen:      *listen,
		Upstream:    target,
		Token:       *token,
		Store:       store,
		Logger:      log.New(stderr, "buffy ", log.LstdFlags),
		Spend:       proxy.NewSpendGuard(proxy.SpendLimits{SessionTokens: *maxSession, DayTokens: *maxDay}),
		Loops:       proxy.LoopLimits{Warn: *loopWarn, Block: *loopBlock},
		Breaker:     proxy.NewBreaker(proxy.BreakerSettings{Failures: *breakerFailures, Cooldown: *breakerCooldown}),
		ContextEdit: contextEdit,
		Retries:     proxy.RetrySettings{Attempts: *retries, BaseDelay: *retryBase, MaxDelay: *retryMax},
	})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		addr := srv.Addr()
		// Best-effort banner: the server runs whether or not stdout is
		// writable.
		if contextEdit != nil {
			_, _ = fmt.Fprintf(stdout, "policy: %s (experimental; applied to sessions whose client enables %s)\n", contextEdit, policy.BetaFeature)
		}
		_, _ = fmt.Fprintf(stdout, "buffy serve listening on http://%s -> %s\nledger: %s\n\nPoint your agent at it:\n  export ANTHROPIC_BASE_URL=http://%s\n\nThen analyze measured data with:\n  buffy replay %s\n\nStop with Ctrl-C. Disable without uninstalling: %s=1.\n", addr, target, dir, addr, dir, envDisabled)
	}()
	return srv.ListenAndServe(ctx)
}

// contextEditFromFlags builds the live policy, or nil when it is off or
// the environment forbids policies.
func contextEditFromFlags(trigger, keep int, forbidden bool) (*policy.ContextEdit, error) {
	if trigger == 0 || forbidden {
		return nil, nil
	}
	p := &policy.ContextEdit{TriggerTokens: trigger, KeepLast: keep}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return p, nil
}

func defaultLedgerDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory for the ledger: %w", err)
	}
	return filepath.Join(home, ".buffy", ledgerDirName), nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
