package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// maxArchive caps what will be read from the network. A release archive is
// about 8 MB; 128 MB is well clear of that and still bounds the damage if the
// origin serves something unexpected. Without it, io.ReadAll on a hostile or
// broken response is unbounded.
const maxArchive = 128 << 20

// Client fetches releases. The zero value is not usable; use NewClient.
type Client struct {
	// HTTP is the transport. Nil means a client with a sane timeout.
	HTTP *http.Client
	// ReleasesBase is the release origin, "https://github.com/<repo>" in
	// production. Tests point it at an httptest server.
	ReleasesBase string
}

// NewClient returns a Client aimed at the real release origin.
func NewClient() *Client {
	return &Client{ReleasesBase: "https://github.com/" + Repo}
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 60 * time.Second}
}

// base is the release origin with any trailing slash removed.
func (c *Client) base() string {
	b := c.ReleasesBase
	for len(b) > 0 && b[len(b)-1] == '/' {
		b = b[:len(b)-1]
	}
	if b == "" {
		b = "https://github.com/" + Repo
	}
	return b
}

// Latest resolves the newest published tag.
//
// It reads the Location header of /releases/latest rather than calling
// api.github.com. That endpoint allows 60 unauthenticated requests an hour per
// IP, which install.sh documents as routinely exhausted on CI and behind shared
// NAT; inheriting that limit would make the upgrade path fail hardest exactly
// when a fleet upgrades together. The redirect is plain github.com and carries
// no such budget.
func (c *Client) Latest(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	hc := *c.httpClient()
	// Do not follow: the answer IS the redirect target.
	hc.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach %s: %w", c.base(), err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("%s/releases/latest returned %s with no redirect; there may be no published release yet",
			c.base(), resp.Status)
	}
	return TagFromLocation(loc)
}

// get retrieves one release asset, bounded by maxArchive.
func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxArchive))
}

// Fetch downloads the release archive for one platform, verifies it against the
// release checksums, and returns the unpacked binary.
//
// Nothing is written to disk here. The caller gets bytes that have already been
// proved to match the published digest, and Apply decides where they land.
//
// The verification mirrors install.sh exactly, because an upgrade path that
// checked less than the installer would let someone weaken their own guarantees
// by using the tool the convenient way.
func (c *Client) Fetch(ctx context.Context, tag, goos, goarch string) ([]byte, error) {
	name := ArchiveName(tag, goos, goarch)
	base := c.base() + "/releases/download/" + tag

	archive, err := c.get(ctx, base+"/"+name)
	if err != nil {
		return nil, fmt.Errorf("no build for %s/%s in %s: %w", goos, goarch, tag, err)
	}

	// A checksums file that cannot be fetched is not a reason to proceed. There
	// is no --no-verify here on purpose: the flag exists in install.sh for
	// bootstrapping, and a machine that already runs replay is past that.
	sums, err := c.get(ctx, base+"/checksums.txt")
	if err != nil {
		return nil, fmt.Errorf("checksums.txt could not be fetched from %s, so the download cannot be verified. Nothing was installed: %w", base, err)
	}
	want, err := DigestFor(sums, name)
	if err != nil {
		return nil, fmt.Errorf("%w: %s. Nothing was installed", err, name)
	}
	sum := sha256.Sum256(archive)
	if got := hex.EncodeToString(sum[:]); got != want {
		return nil, fmt.Errorf("checksum mismatch for %s. Nothing was installed.\nchecksums.txt says %s, the download hashes to %s", name, want, got)
	}

	// The hash proves the archive matches checksums.txt. This proves
	// checksums.txt is the one this project's CI signed.
	if err := c.verifyChecksumSignature(ctx, base, sums); err != nil {
		return nil, err
	}

	return unpack(archive)
}

// unpack pulls the single binary out of the release tarball.
//
// A member that is a symlink named `replay` would otherwise be followed and
// installed as a 0755 executable holding whatever it pointed at on this
// machine. install.sh rejects that case explicitly and this does too.
func unpack(archive []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("the release archive is not gzip: %w", err)
	}
	defer func() { _ = zr.Close() }()

	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("the release archive is not readable: %w", err)
		}
		if filepath.Base(hdr.Name) != Binary {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeReg:
			b, err := io.ReadAll(io.LimitReader(tr, maxArchive))
			if err != nil {
				return nil, err
			}
			return b, nil
		case tar.TypeSymlink, tar.TypeLink:
			return nil, fmt.Errorf("the archive contains a symlink where %s should be. Nothing was installed", Binary)
		default:
			return nil, fmt.Errorf("the archive entry for %s is not a regular file. Nothing was installed", Binary)
		}
	}
	return nil, fmt.Errorf("the archive did not contain a %s binary", Binary)
}

// Apply installs bin at dest, replacing whatever is there.
//
// It stages a sibling, runs it, and only then moves it into place. Whatever is
// already at dest is, as far as this code knows, a working install: overwriting
// it before the replacement has been proved to run means a bad upgrade takes
// the user's working copy with it, and removing the broken file afterwards is
// not a fix, it just makes the loss tidy.
//
// The final rename is atomic within a filesystem, so there is no window where
// dest holds a half-written file. The staged name shares dest's directory for
// that reason — a temp dir on another filesystem would turn the rename into a
// copy and reopen the window.
func Apply(bin []byte, dest string) error {
	dir := filepath.Dir(dest)
	staged := filepath.Join(dir, "."+filepath.Base(dest)+fmt.Sprintf(".new.%d", os.Getpid()))

	if err := os.WriteFile(staged, bin, 0o755); err != nil {
		return fmt.Errorf("could not write to %s: %w", dir, err)
	}
	// Any failure from here on must take the staged file with it.
	defer func() { _ = os.Remove(staged) }()

	// Run it once before claiming success. Everything above verifies the bytes
	// that arrived; none of it proves the result executes here. A binary for
	// the wrong architecture, a chmod that did not take, a libc mismatch: each
	// installs cleanly, fails on first use, and would be reported as "upgraded"
	// either way.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, staged, "version").CombinedOutput()
	if err != nil {
		detail := ""
		if s := bytes.TrimSpace(out); len(s) > 0 {
			detail = ": " + string(bytes.SplitN(s, []byte("\n"), 2)[0])
		}
		if _, statErr := os.Stat(dest); statErr == nil {
			return fmt.Errorf("the downloaded binary does not run on this machine%s. Nothing was changed; your existing %s is untouched", detail, dest)
		}
		return fmt.Errorf("the downloaded binary does not run on this machine%s. Nothing was installed", detail)
	}

	if err := os.Rename(staged, dest); err != nil {
		return fmt.Errorf("could not move the new binary into %s. Nothing was changed: %w", dir, err)
	}
	// The rename consumed it; stop the deferred remove from deleting dest's
	// predecessor path, which no longer exists anyway.
	return nil
}
