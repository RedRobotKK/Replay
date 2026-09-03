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

	"github.com/RedRobotKK/Buffy/internal/ledger"
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
)

// errDisabled is returned when the kill switch is set.
var errDisabled = errors.New(envDisabled + " is set; Buffy will not start. To bypass Buffy entirely, unset ANTHROPIC_BASE_URL")

func runServe(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	listen := fs.String("listen", defaultListen, "loopback address to bind")
	upstream := fs.String("upstream", envOr(envUpstream, defaultUpstream), "provider base URL")
	ledgerDir := fs.String("ledger", "", "ledger directory (default ~/.buffy/ledger)")
	token := fs.String("token", os.Getenv(envToken), "require this value in the "+proxy.HeaderToken+" header")
	maxSession := fs.Int("max-session-tokens", 0, "refuse a session's next request once it has consumed this many tokens (0 = off)")
	maxDay := fs.Int("max-day-tokens", 0, "refuse requests once this many tokens were consumed today, UTC (0 = off)")
	loopWarn := fs.Int("loop-warn", 0, "add a warning header when one identical tool call repeats this many times (0 = off)")
	loopBlock := fs.Int("loop-block", 0, "refuse the request when one identical tool call repeats this many times (0 = off)")
	breakerFailures := fs.Int("breaker-failures", 0, "open the circuit after this many consecutive provider failures (0 = off)")
	breakerCooldown := fs.Duration("breaker-cooldown", defaultBreakerCooldown, "how long the circuit stays open")
	if err := fs.Parse(args); err != nil {
		return errUsage
	}
	if os.Getenv(envDisabled) != "" {
		return errDisabled
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
		Listen:   *listen,
		Upstream: target,
		Token:    *token,
		Store:    store,
		Logger:   log.New(stderr, "buffy ", log.LstdFlags),
		Spend:    proxy.NewSpendGuard(proxy.SpendLimits{SessionTokens: *maxSession, DayTokens: *maxDay}),
		Loops:    proxy.LoopLimits{Warn: *loopWarn, Block: *loopBlock},
		Breaker:  proxy.NewBreaker(proxy.BreakerSettings{Failures: *breakerFailures, Cooldown: *breakerCooldown}),
	})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		addr := srv.Addr()
		_, _ = fmt.Fprintf(stdout, "buffy serve listening on http://%s -> %s\nledger: %s\n\nPoint your agent at it:\n  export ANTHROPIC_BASE_URL=http://%s\n\nThen analyze measured data with:\n  buffy replay %s\n\nStop with Ctrl-C. Disable without uninstalling: %s=1.\n", addr, target, dir, addr, dir, envDisabled)
	}()
	return srv.ListenAndServe(ctx)
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
