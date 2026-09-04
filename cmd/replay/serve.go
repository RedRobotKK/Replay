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
	"strconv"
	"syscall"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/masking"
	"github.com/RedRobotKK/Replay/internal/policy"
	"github.com/RedRobotKK/Replay/internal/proxy"
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
	envDisabled = "REPLAY_DISABLED"
	envToken    = "REPLAY_TOKEN"
	envUpstream = "REPLAY_UPSTREAM"
	// envNoPolicy turns every live policy off without touching flags, so
	// a user can rule Replay's edits out in one move while keeping the
	// ledger and the guards.
	envNoPolicy = "REPLAY_NO_POLICY"
)

// errDisabled is returned when the kill switch is set.
var errDisabled = errors.New(envDisabled + " is set; Replay will not start. To bypass Replay entirely, unset ANTHROPIC_BASE_URL")

func runServe(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	listen := fs.String("listen", defaultListen, "loopback address to bind")
	upstream := fs.String("upstream", envOr(envUpstream, defaultUpstream), "provider base URL")
	ledgerDir := fs.String("ledger", "", "ledger directory (default ~/.replay/ledger)")
	token := fs.String("token", "", "require this value in the "+proxy.HeaderToken+" header (or set "+envToken+")")
	maxSession := fs.Int("max-session-tokens", 0, "refuse a session's next request once it has consumed this many tokens (0 = off)")
	maxDay := fs.Int("max-day-tokens", 0, "refuse requests once this many tokens were consumed today, UTC (0 = off)")
	maxSessionUSD := fs.Float64("max-session-usd", 0, "refuse a session's next request once its list-price cost reaches this many dollars (0 = off; models not in the price table count as free)")
	maxDayUSD := fs.Float64("max-day-usd", 0, "refuse requests once today's list-price cost reaches this many dollars, UTC (0 = off)")
	errorBudget := fs.Float64("error-budget", 0, "refuse a session's next request once this share of its prompt tokens carried error content, e.g. 0.3 (0 = off)")
	loopWarn := fs.Int("loop-warn", 0, "add a warning header when one identical tool call repeats this many times (0 = off)")
	loopBlock := fs.Int("loop-block", 0, "refuse the request when one identical tool call repeats this many times (0 = off)")
	breakerFailures := fs.Int("breaker-failures", 0, "open the circuit after this many consecutive provider failures (0 = off)")
	breakerCooldown := fs.Duration("breaker-cooldown", defaultBreakerCooldown, "how long the circuit stays open")
	holdSiblings := fs.Duration("hold-siblings", 0, "hold a request whose tools and system prompt are already in flight and not yet cached until that first response begins, so parallel sub-agents read the cache instead of all writing it; the value is the longest wait (0 = off; suggested "+proxy.DefaultSiblingWait.String()+")")
	retries := fs.Int("retries", 0, "resend a request up to this many times on rate limit, overload, server error, or connection failure, before any response byte reached the client (0 = off)")
	retryBase := fs.Duration("retry-base", proxy.DefaultRetryBaseDelay, "first retry backoff; doubles with jitter, capped by -retry-max; a provider Retry-After within the cap replaces it")
	retryMax := fs.Duration("retry-max", proxy.DefaultRetryMaxDelay, "longest wait before a retry")
	editTrigger := fs.Int("context-edit-trigger", 0, "EXPERIMENTAL: ask the provider to clear old tool results once the prompt passes this many tokens, on requests whose client enabled "+policy.BetaFeature+" and set no context_management of its own (0 = off; "+envNoPolicy+"=1 forces off)")
	editKeep := fs.Int("context-edit-keep", analysis.ContextEditKeepLast, "how many recent tool results a clear keeps")
	trialShare := fs.Float64("trial-share", proxy.DefaultTrialShare, "share of new sessions that get the policy from -policy-file; the rest run as controls (stable per session id)")
	guardrail := fs.Float64("guardrail-reread", 0, "revert the policy from -policy-file for new sessions once treated sessions' re-read rate after the provider's first clear reaches this share (0 = off)")
	revertAfter := fs.Int("revert-after", proxy.DefaultRevertAfter, "how many sessions must breach the guardrail before the policy is reverted")
	mask := fs.Bool("mask", false, "EXPERIMENTAL: replace secrets matching the named pattern set with vault placeholders before requests leave the machine, and restore them in responses within -rehydrate-scope (see README)")
	maskPatterns := fs.String("mask-patterns", "", "file of user-defined patterns for -mask, one per line as name<TAB>regexp")
	maskEntropy := fs.Bool("mask-entropy", false, "with -mask, also mask runs that look like credentials by shape and entropy. Needs mixed case and digits over "+strconv.Itoa(masking.EntropyMinLength)+" characters, so bare hex and lowercase secrets are NOT caught by shape; those are caught only when a name like TOKEN= or api_key: sits beside them. Reported as pattern "+masking.EntropyPattern)
	rehydrate := fs.Bool("rehydrate", true, "with -mask, restore placeholders in responses; false leaves them in place to evaluate coverage")
	project := fs.String("project", "", "with -mask, the directory under which file-edit tool inputs may receive secrets (default: the current directory)")
	var scopeSpecs []string
	fs.Func("rehydrate-scope", "with -mask, where a pattern's secrets may be restored, as name=dest[,dest] with dest text, edit, tool:NAME, or none; name * sets the default (text,edit); repeatable", func(v string) error {
		scopeSpecs = append(scopeSpecs, v)
		return nil
	})
	policyFile := fs.String("policy-file", "", "EXPERIMENTAL: apply the context-edit candidate selected by replay learn (usually ~/.replay/policy.json), read at each session's first request; an explicit -context-edit-trigger wins; a session keeps its first decision whatever the file does later")
	if err := fs.Parse(args); err != nil {
		return errUsage
	}
	noPolicy := os.Getenv(envNoPolicy) != ""
	contextEdit, err := contextEditFromFlags(*editTrigger, *editKeep, noPolicy)
	if err != nil {
		return err
	}
	if noPolicy {
		*policyFile = ""
		*mask = false
	}
	masker, rehydrator, err := maskingFromFlags(*mask, *maskPatterns, *rehydrate, *project, scopeSpecs)
	if err != nil {
		return err
	}
	if masker != nil {
		masker.Entropy = *maskEntropy
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
		Logger:      log.New(stderr, "replay ", log.LstdFlags),
		Spend:       proxy.NewSpendGuard(proxy.SpendLimits{SessionTokens: *maxSession, DayTokens: *maxDay, SessionUSD: *maxSessionUSD, DayUSD: *maxDayUSD}),
		Loops:       proxy.LoopLimits{Warn: *loopWarn, Block: *loopBlock},
		Breaker:     proxy.NewBreaker(proxy.BreakerSettings{Failures: *breakerFailures, Cooldown: *breakerCooldown}),
		ContextEdit: contextEdit,
		PolicyFile:  *policyFile,
		NoPolicy:    noPolicy,
		Masker:      masker,
		Rehydrator:  rehydrator,
		Trial:       proxy.TrialSettings{Share: *trialShare, ReReadRate: *guardrail, RevertAfter: *revertAfter},
		Siblings:    proxy.SiblingSettings{MaxWait: *holdSiblings},
		Retries:     proxy.RetrySettings{Attempts: *retries, BaseDelay: *retryBase, MaxDelay: *retryMax},
		ErrorBudget: proxy.ErrorBudget{Share: *errorBudget},
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
		} else if *policyFile != "" {
			_, _ = fmt.Fprintf(stdout, "policy: from %s at each session's first request (experimental)\n", *policyFile)
		}
		if masker != nil && rehydrator != nil {
			_, _ = fmt.Fprintf(stdout, "masking: on (experimental); rehydration scope %s, project %s\n", rehydrator.Scopes().Default, rehydrator.Scopes().Project)
		} else if masker != nil {
			_, _ = fmt.Fprintf(stdout, "masking: on (experimental); rehydration off, placeholders stay in responses\n")
		}
		_, _ = fmt.Fprintf(stdout, "replay serve listening on http://%s -> %s\nledger: %s\n\nPoint your agent at it:\n  export ANTHROPIC_BASE_URL=http://%s\n\nThen analyze measured data with:\n  replay replay %s\n\nStop with Ctrl-C. Disable without uninstalling: %s=1.\n", addr, target, dir, addr, dir, envDisabled)
	}()
	return srv.ListenAndServe(ctx)
}

// vaultDirName is where the masking vault lives under ~/.replay.
const vaultDirName = "vault"

// maskingFromFlags opens the vault and builds the masker and, unless
// rehydration is off, the rehydrator; both nil when masking is off.
func maskingFromFlags(on bool, patternsFile string, rehydrate bool, project string, scopeSpecs []string) (*masking.Masker, *masking.Rehydrator, error) {
	if !on {
		return nil, nil, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, fmt.Errorf("find home directory for the vault: %w", err)
	}
	vault, err := masking.OpenVault(filepath.Join(home, ".replay", vaultDirName))
	if err != nil {
		return nil, nil, err
	}
	var user []masking.Pattern
	if patternsFile != "" {
		text, err := os.ReadFile(patternsFile)
		if err != nil {
			return nil, nil, fmt.Errorf("read mask patterns: %w", err)
		}
		user, err = masking.ParseUserPatterns(string(text))
		if err != nil {
			return nil, nil, err
		}
	}
	masker := masking.New(vault, user)
	if !rehydrate {
		return masker, nil, nil
	}
	if project == "" {
		project, err = os.Getwd()
		if err != nil {
			return nil, nil, fmt.Errorf("find the project directory: %w", err)
		}
	}
	project, err = masking.ResolveRoot(project)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve the project directory: %w", err)
	}
	scopes, err := masking.ParseScopes(project, scopeSpecs, append(append([]masking.Pattern(nil), masking.Patterns...), user...))
	if err != nil {
		return nil, nil, err
	}
	return masker, masking.NewRehydrator(vault, scopes), nil
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
	return filepath.Join(home, ".replay", ledgerDirName), nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
