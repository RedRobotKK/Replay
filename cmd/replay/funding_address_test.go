package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A published payment address is the one string in this repository where a
// single wrong character sends money to a stranger, permanently, with no
// recourse. It is also a known target: changing one character inside an
// otherwise plausible pull request is a documented attack on open-source
// projects, and it survives review precisely because a 42-character hex string
// is exactly what a reviewer's eye slides over.
//
// So the address is pinned here. Any change to it, in any file, has to change
// this constant too — which puts the diff somewhere a reviewer will actually
// look, next to a comment explaining why.
//
// EIP-55 checksum verified 2026-09-05 with an implementation self-tested
// against the specification's own vectors before it was trusted on this
// address. The mixed case is not decoration: it IS the checksum, so a
// lowercased copy is a weaker string and should not be substituted for it.
const fundingAddress = "0x585ef883e750694E4ba1463bc20820e9C4fBF369"

// The network is part of the address for practical purposes. USDC exists on
// several chains under the same address, they are not interchangeable, and
// naming the wrong one loses somebody's money.
const fundingNetwork = "Avalanche C-Chain"

// Bitcoin, P2SH. Base58Check verified 2026-09-05 with a decoder self-tested on
// known-good addresses first. Base58 has no 0, O, I or l precisely so that a
// human transcribing it cannot produce a valid-looking wrong address — but a
// machine editing a file has no such difficulty, which is why this is pinned
// too.
const fundingAddressBTC = "3HzfvNb1iKjeKsRMgMSttP1oqJzyHULhGu"

// Every address published anywhere in this repository. An EVM-shaped or
// Bitcoin-shaped string in the documentation must be one of these.
var fundingAddresses = map[string]string{
	"0x585ef883e750694E4ba1463bc20820e9C4fBF369": "USDC on Avalanche C-Chain",
	// The x402 receiving wallet. It is separate from the donation addresses on
	// purpose: donations are gifts, and this one takes payments for a product,
	// so keeping them apart keeps the accounting honest and lets either be
	// rotated without disturbing the other. EIP-55 verified 2026-09-05.
	"0x2733E9BE752848D578937fDB6029D7c739dc89Cb": "x402 receiving wallet, USDC on Base",
	// Not a destination — the USDC token contract on Base, which the x402
	// terms name as the asset. It is pinned for the same reason as the
	// destinations: a buyer who pays the right address in the wrong token has
	// still lost the money, and swapping a token contract for a worthless
	// lookalike is the same attack wearing different clothes. Canonical
	// Circle USDC on Base, EIP-55 verified 2026-09-05.
	"0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913": "USDC token contract, the asset rather than a destination, on Base",
	"0xdaC0fCFa02b20aF55e6e34e931fB169a0C8Ddb98": "cbBTC on Base",
	"3HzfvNb1iKjeKsRMgMSttP1oqJzyHULhGu":         "BTC on Bitcoin",
	// Solana carries no checksum, so this one is pinned and nothing else can
	// verify it. That is exactly why pinning it matters more, not less.
	"F7XcHFFGe4uJUTrQJUELwfC4VzPYNvy9th1Yx3jVz6zc": "BTC on Solana",
}

func TestFundingAddressIsPinned(t *testing.T) {
	roots := []string{"README.md", "FUNDING.md", "docs"}
	var found int

	var scan func(string)
	scan = func(path string) {
		info, err := os.Stat(path)
		if err != nil {
			return
		}
		if info.IsDir() {
			entries, _ := os.ReadDir(path)
			for _, e := range entries {
				scan(filepath.Join(path, e.Name()))
			}
			return
		}
		if !strings.HasSuffix(path, ".md") {
			return
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return
		}
		text := string(b)

		// Any Bitcoin-shaped string must be exactly the pinned one.
		for _, tok := range strings.Fields(strings.NewReplacer("`", " ", "(", " ", ")", " ", ",", " ").Replace(text)) {
			if len(tok) < 26 || len(tok) > 35 {
				continue
			}
			if tok[0] != '1' && tok[0] != '3' {
				continue
			}
			if strings.ContainsAny(tok, "0OIl") || !isBase58(tok) {
				continue
			}
			if _, ok := fundingAddresses[tok]; !ok {
				t.Errorf("%s contains a Bitcoin-shaped address that is not the pinned one:\n"+
					"  found: %s\nIf intended, add it to fundingAddresses and say why in the commit.", path, tok)
			} else {
				found++
			}
		}

		// Solana: 43-44 Base58 characters, no checksum to lean on.
		for _, tok := range strings.Fields(strings.NewReplacer("`", " ", "|", " ", ",", " ").Replace(text)) {
			if len(tok) < 43 || len(tok) > 44 || !isBase58(tok) {
				continue
			}
			if _, ok := fundingAddresses[tok]; !ok {
				t.Errorf("%s contains a Solana-shaped address that is not pinned:\n  found: %s", path, tok)
			} else {
				found++
			}
		}

		// Any EVM-shaped string in the documentation must be exactly this one.
		for _, tok := range strings.Fields(strings.NewReplacer("`", " ", "(", " ", ")", " ", ",", " ").Replace(text)) {
			if !strings.HasPrefix(tok, "0x") || len(tok) != 42 {
				continue
			}
			if _, ok := fundingAddresses[tok]; !ok {
				t.Errorf("%s contains an EVM address that is not the pinned one:\n"+
					"  found: %s\n"+
					"If this change is intended, add it to fundingAddresses and say why in the "+
					"commit. If it is not, somebody is trying to redirect donations.",
					path, tok)
			} else {
				found++
			}
		}
	}
	for _, r := range roots {
		scan("../../" + r)
	}

	if found == 0 {
		t.Error("the funding address appears in no documentation; either it was " +
			"removed, or this test is looking in the wrong place and is now useless")
	}

	// Every address must be published beside the network it belongs to. The
	// same token on the wrong chain is lost, and an address without a network
	// is an invitation to lose it.
	funding, err := os.ReadFile("../../FUNDING.md")
	if err != nil {
		t.Fatalf("FUNDING.md is where the addresses live and it is unreadable: %v", err)
	}
	for addr, network := range fundingAddresses {
		text := string(funding)
		if !strings.Contains(text, addr) {
			t.Errorf("%s (%s) is pinned but published nowhere", addr, network)
			continue
		}
		// The network name has to appear, or the address stands alone.
		want := strings.Fields(network)[len(strings.Fields(network))-1]
		if !strings.Contains(text, want) {
			t.Errorf("%s is published without naming its network (%q)", addr, network)
		}
	}
}

// The mixed case is the EIP-55 checksum. A lowercased address is still valid to
// send to, but it carries no error detection, so publishing one throws away the
// only protection a reader has against a typo.
func TestFundingAddressKeepsItsChecksum(t *testing.T) {
	if fundingAddress == strings.ToLower(fundingAddress) {
		t.Error("the pinned address is all lowercase, so it carries no EIP-55 checksum")
	}
	if fundingAddress == strings.ToUpper(fundingAddress) {
		t.Error("the pinned address is all uppercase, so it carries no EIP-55 checksum")
	}
}

func isBase58(s string) bool {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	for _, c := range s {
		if !strings.ContainsRune(alphabet, c) {
			return false
		}
	}
	return true
}
