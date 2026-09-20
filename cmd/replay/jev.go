package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// runJev prints what a Jev capture holds, and nothing it does not.
//
// This is an inspection surface, not an analysis one. A Jev capture reports
// input and output tokens and no cache fields at all, so the question Replay
// exists to answer cannot be asked of it: ADR-0019 puts the distinction
// between a cache write and a cache read at the centre of that measurement,
// and a surface without it "cannot answer the question, however cleanly its
// responses parse". The honest reading is therefore to show the observation
// and stop, rather than to route it through accounting that would emit a zero
// nobody measured.
//
// It also never builds a transcript.Session. That is deliberate and is the
// reason this command exists at all: Source.Tier() is a two-valued provenance
// label, and admitting a server-side capture to it would be a provenance claim
// no decision has authorised.
func runJev(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("jev", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		// No default location is searched, because there is not one to search.
		// A Jev capture is Replay's own artifact arriving on a stream rather
		// than a file some vendor left in a home directory, so guessing a path
		// would be inventing a convention rather than following one.
		_, _ = fmt.Fprintf(stderr, "replay: jev needs a capture path\n\n"+
			"  replay jev <capture.jsonl> [more.jsonl...]\n\n"+
			"  A Jev capture is written by Replay, not by a vendor, so there is no\n"+
			"  default location to look in.\n")
		return errUsage
	}

	// Every requested file is read before anything is printed.
	//
	// The reader refuses a malformed record outright, and a command that
	// printed the files which happened to parse would turn that refusal into a
	// footnote beside a report a reader cannot distinguish from a complete
	// one. So a single failure fails the invocation and prints no report.
	var buf bytes.Buffer
	for _, path := range fs.Args() {
		evals, err := transcript.ParseJevCaptureFile(path)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "replay: %s: %v\n", path, err)
			return fmt.Errorf("jev: %s: %w", path, err)
		}
		for _, ev := range evals {
			writeJevEvaluation(&buf, ev)
		}
	}

	_, _ = fmt.Fprintf(stdout, "\n  Jev capture\n\n")
	_, err := stdout.Write(buf.Bytes())
	return err
}

// writeJevEvaluation renders one evaluation and the attempts it owns.
//
// The nesting is the contract: one evaluation owns every attempt of one
// invocation, so attempts are indented under the identity rather than printed
// as rows of their own. Flattening them would make a retried evaluation read
// as several.
func writeJevEvaluation(w io.Writer, ev transcript.JevEvaluation) {
	_, _ = fmt.Fprintf(w, "  %s\n", ev.EvaluationID)
	// Requested and resolved are kept apart. The alias the caller asked for and
	// the build that answered are two facts, and the resolution is observable
	// nowhere else.
	_, _ = fmt.Fprintf(w, "    requested  %s\n", ev.RequestedModel)
	if m := jevResolvedModel(ev); m != "" {
		_, _ = fmt.Fprintf(w, "    resolved   %s\n", m)
	}

	for _, a := range ev.Attempts {
		_, _ = fmt.Fprintf(w, "    attempt %d  HTTP %d", a.Ordinal, a.HTTPStatus)
		// The provider's id is per-attempt, optional, and never the identity of
		// anything. It is printed only where the header actually carried one,
		// because a blank column reads as an observation rather than an absence.
		if a.RequestIDMeasured {
			_, _ = fmt.Fprintf(w, "  request id %s", a.ProviderRequestID)
		}
		_, _ = fmt.Fprintln(w)

		if a.Response == nil {
			// Said outright. A zero-valued usage line here would read as an
			// evaluation that succeeded and consumed nothing.
			_, _ = fmt.Fprintf(w, "      no response\n")
			continue
		}
		writeJevResponse(w, a.Response)
	}
	_, _ = fmt.Fprintln(w)
}

// jevResolvedModel returns the model that answered, when one did.
func jevResolvedModel(ev transcript.JevEvaluation) string {
	for _, a := range ev.Attempts {
		if a.Response != nil {
			return a.Response.Model
		}
	}
	return ""
}

// writeJevResponse prints the provider's own figures, unaltered.
func writeJevResponse(w io.Writer, r *transcript.JevResponse) {
	// Token counts are the provider's observation and are printed as such. No
	// rate is derived from them and nothing is divided by anything: this
	// surface has no rules document, no calibration and no prefix evidence, so
	// any figure beyond the two counts would be manufactured.
	_, _ = fmt.Fprintf(w, "      tokens %d in, %d out\n", r.Usage.InputTokens, r.Usage.OutputTokens)

	keys := make([]string, 0, len(r.Answers))
	for k := range r.Answers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		writeJevAnswer(w, k, r.Answers[k])
	}
}

// writeJevAnswer renders one member of the closed answer union.
//
// The three forms stay apart. A noul carries a probability and no confidence,
// and giving it one here would put a defaulted 0.0 beside the measured ones
// the provider does report.
func writeJevAnswer(w io.Writer, key string, a transcript.JevAnswer) {
	switch v := a.(type) {
	case transcript.JevNoul:
		_, _ = fmt.Fprintf(w, "      %s  noul %s\n", key, jevNum(v.Noul))
	case transcript.JevChoice:
		_, _ = fmt.Fprintf(w, "      %s  choice %s  confidence %s\n", key, v.Choice, jevNum(v.Confidence))
		writeJevFloatMap(w, "probabilities", v.Probabilities)
	case transcript.JevScore:
		// The score is an expected value over the rubric and is printed as one.
		// It is not an index into the legend: the observed 0.14 lies between
		// levels, and reading it as an index would report level 0 silently.
		_, _ = fmt.Fprintf(w, "      %s  score %s  confidence %s\n", key, jevNum(v.Score), jevNum(v.Confidence))
		writeJevStringMap(w, "legend", v.Legend)
		writeJevFloatMap(w, "probabilities", v.Probabilities)
	}
}

func writeJevFloatMap(w io.Writer, label string, m map[string]float64) {
	if len(m) == 0 {
		return
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	_, _ = fmt.Fprintf(w, "        %s", label)
	for _, k := range keys {
		// Printed one by one, exactly as the provider sent them. They are not
		// summed, renormalised or rounded: they are the provider's distribution
		// and arithmetic over them here would be this build's opinion.
		_, _ = fmt.Fprintf(w, "  %s %s", k, jevNum(m[k]))
	}
	_, _ = fmt.Fprintln(w)
}

func writeJevStringMap(w io.Writer, label string, m map[string]string) {
	if len(m) == 0 {
		return
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	_, _ = fmt.Fprintf(w, "        %s", label)
	for _, k := range keys {
		_, _ = fmt.Fprintf(w, "  %s %s", k, m[k])
	}
	_, _ = fmt.Fprintln(w)
}

// jevNum formats a float without changing it.
//
// 'g' with precision -1 emits the shortest decimal that reads back as the same
// float64, so a printed value round-trips to the one the provider sent. A fixed
// precision would quietly round a probability.
func jevNum(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }
