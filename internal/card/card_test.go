package card

import (
	"bytes"
	"image/png"
	"reflect"
	"strings"
	"testing"
)

func sample() Data {
	return Data{
		AvoidableShare:      0.031,
		Tasks:               64,
		Breaks:              4,
		MedianUSD:           0.20,
		P90USD:              0.40,
		PeakAvoidableTokens: 189_000,
	}
}

// The card is posted to services that crop anything else. 1200x630 is the
// Open Graph / Twitter summary_large_image size, and a card that is a few
// pixels off is re-encoded and re-cropped by the platform, which is where the
// install line loses its last characters.
//
// PASS: the encoded PNG decodes to exactly 1200x630 for every variant.
// FAIL: any other size, including a merely proportional one.
func TestRenderedCardIsExactlyOpenGraphSize(t *testing.T) {
	for _, v := range Variants() {
		var buf bytes.Buffer
		if err := Encode(&buf, v, ToneMeasured, sample()); err != nil {
			t.Fatalf("card %s: %v", v, err)
		}
		img, err := png.Decode(&buf)
		if err != nil {
			t.Fatalf("card %s does not decode as PNG: %v", v, err)
		}
		b := img.Bounds()
		if b.Dx() != 1200 || b.Dy() != 630 {
			t.Errorf("card %s is %dx%d, want 1200x630", v, b.Dx(), b.Dy())
		}
	}
}

// The install line is the only thing on the card that does anything, and the
// arm letter in its path is the only thing that ties an install back to the
// design that produced it. Assert the string, then assert the pixels: a
// constant that is right while the renderer draws something else passes the
// first check alone, and the card is a picture, not a string.
func TestInstallLineCarriesTheVariantInPixels(t *testing.T) {
	want := map[Variant]string{
		VariantB: "curl -fsSL https://redrobot.jp/c/b | sh",
		VariantC: "curl -fsSL https://redrobot.jp/c/c | sh",
	}
	for v, line := range want {
		if got := InstallLine(v); got != line {
			t.Errorf("install line for %s:\n got %q\nwant %q", v, got, line)
		}
		img, err := Render(v, ToneMeasured, sample())
		if err != nil {
			t.Fatalf("card %s: %v", v, err)
		}
		hays := cardMasks(img)
		if !find(t, hays, v, line) {
			t.Errorf("card %s does not actually carry %q in its pixels", v, line)
		}
		// The wrong arm must not be findable, or the two cards are the same
		// card and the comparison they exist for cannot be read. Matched on the
		// fragment that differs: one character of a 39-character line is inside
		// the antialiasing tolerance, so the whole line cannot tell them apart.
		if find(t, hays, v, "/c/"+string(otherArm(v))+" ") {
			t.Errorf("card %s carries the other arm's path /c/%s", v, otherArm(v))
		}
		if !find(t, hays, v, "/c/"+string(v)+" ") {
			t.Errorf("card %s does not carry its own arm's path /c/%s", v, v)
		}
	}
}

// The install line must survive being retyped from a photograph of a phone
// screen, so it carries nothing the shell would need protecting from.
//
// The earlier form was `curl -fsSL "https://redrobot.jp/replay.sh?src=card&v=b"
// | sh`. zsh is the macOS default shell and globs a bare ?, aborting with "no
// matches found" before curl ever runs, so that URL had to be quoted — and the
// quotes are then two more characters to get right by hand. Moving attribution
// into the path removes the cause rather than working around it.
func TestInstallLineHasNoShellMetacharacters(t *testing.T) {
	for _, v := range Variants() {
		line := InstallLine(v)
		for _, bad := range []string{`"`, "'", "?", "&", "$", "*", "\\", ";", "`"} {
			if strings.Contains(line, bad) {
				t.Errorf("card %s: the install line contains %q, which a shell or a "+
					"person retyping it will get wrong: %q", v, bad, line)
			}
		}
		// The pipe is the one metacharacter that has to be there, and it is
		// what makes the line an install rather than a download.
		if !strings.Contains(line, "| sh") {
			t.Errorf("card %s: %q does not pipe to a shell", v, line)
		}
		if !strings.HasPrefix(line, "curl -fsSL https://") {
			t.Errorf("card %s: %q must keep -fsSL and https. Without -f an HTML error "+
				"page is piped into sh, without -L a redirect ends the install, and "+
				"without https the first hop is plaintext", v, line)
		}
	}
}

// Nothing that carries a filesystem path or a project name can reach the
// renderer, because the renderer has nowhere to put one. This is a structural
// check rather than a search of the output: a search only proves the leak was
// absent for the input the test happened to use, while a type with no string
// field cannot carry a path at all.
func TestCardDataCannotCarryAPathOrAProjectName(t *testing.T) {
	tp := reflect.TypeOf(Data{})
	for i := 0; i < tp.NumField(); i++ {
		f := tp.Field(i)
		switch f.Type.Kind() {
		case reflect.Int, reflect.Float64, reflect.Bool:
		default:
			t.Errorf("Data.%s is %s. The card's input is deliberately numbers only: "+
				"a string field is somewhere a path, a project name or a model id "+
				"could arrive, and the card is posted in public",
				f.Name, f.Type)
		}
	}
}

// These get regenerated — on every run of `cost --share --png`, and by whoever
// re-renders the set after a copy change. Identical input has to produce an
// identical file, or nobody can tell a design change from encoder noise, and
// every regeneration is a diff that has to be reviewed by eye.
func TestRenderIsDeterministic(t *testing.T) {
	for _, v := range Variants() {
		var first, second bytes.Buffer
		if err := Encode(&first, v, ToneMeasured, sample()); err != nil {
			t.Fatal(err)
		}
		if err := Encode(&second, v, ToneMeasured, sample()); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first.Bytes(), second.Bytes()) {
			t.Errorf("card %s: two renders of identical input differ (%d vs %d bytes)",
				v, first.Len(), second.Len())
		}
	}
}

// A number that moved must move the picture. Without this the determinism test
// above is satisfied by a renderer that ignores its input entirely, which is
// the shape of a check that cannot fail.
func TestTheFiguresActuallyReachThePicture(t *testing.T) {
	base := sample()
	moved := base
	moved.AvoidableShare = 0.19
	moved.Breaks = 41
	moved.Tasks = 900
	moved.PeakAvoidableTokens = 12_345
	for _, v := range Variants() {
		var a, b bytes.Buffer
		if err := Encode(&a, v, ToneMeasured, base); err != nil {
			t.Fatal(err)
		}
		if err := Encode(&b, v, ToneMeasured, moved); err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(a.Bytes(), b.Bytes()) {
			t.Errorf("card %s renders identically for different figures; the data "+
				"is not reaching the pixels", v)
		}
	}
}

// The two variants have to be visibly different designs, not one design with a
// letter changed. If they are near-identical the A/B comparison measures
// nothing and would still look like evidence.
func TestTheVariantsAreDifferentDesigns(t *testing.T) {
	b, err := Render(VariantB, ToneMeasured, sample())
	if err != nil {
		t.Fatal(err)
	}
	c, err := Render(VariantC, ToneMeasured, sample())
	if err != nil {
		t.Fatal(err)
	}
	var differ int
	for y := 0; y < 630; y++ {
		for x := 0; x < 1200; x++ {
			if b.RGBAAt(x, y) != c.RGBAAt(x, y) {
				differ++
			}
		}
	}
	if frac := float64(differ) / (1200 * 630); frac < 0.5 {
		t.Errorf("the two variants share %.0f%% of their pixels; they are one design, "+
			"not two", 100*(1-frac))
	}
}

// otherArm names the variant this one is not, so a test can look for the URL
// that must be absent.
func otherArm(v Variant) Variant {
	if v == VariantB {
		return VariantC
	}
	return VariantB
}

// find draws the string through the same path the card uses and looks for that
// bitmap in the card. It answers "is this text really on this picture", which
// is the only question worth asking of a rendered card.
func find(t *testing.T, hays [][][]bool, v Variant, s string) bool {
	t.Helper()
	needles, err := installStamps(v, s)
	if err != nil {
		t.Fatalf("rendering the needle: %v", err)
	}
	return found(hays, needles)
}
