package main

// SCRATCH. Delete before finishing.

import (
	"math/rand"
	"os"
	"testing"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

func benchCorpus(b *testing.B) []string {
	root := os.Getenv("REPLAY_BENCH_CORPUS")
	if root == "" {
		b.Skip("no REPLAY_BENCH_CORPUS")
	}
	files, err := transcriptFiles([]string{root})
	if err != nil {
		b.Fatal(err)
	}
	return files
}

func runPool(b *testing.B, files []string) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = forEachSession(files, func(string, *transcript.Session, *analysis.LaneReport, error) error {
			return nil
		})
	}
}

// Largest-first, as transcriptFiles returns them.
func BenchmarkPoolLargestFirst(b *testing.B) { runPool(b, benchCorpus(b)) }

// Shuffled, to price the largest-first ordering.
func BenchmarkPoolShuffled(b *testing.B) {
	files := benchCorpus(b)
	f := append([]string(nil), files...)
	rand.New(rand.NewSource(1)).Shuffle(len(f), func(i, j int) { f[i], f[j] = f[j], f[i] })
	runPool(b, f)
}

// Smallest-first, the worst case for a bounded window.
func BenchmarkPoolSmallestFirst(b *testing.B) {
	files := benchCorpus(b)
	f := make([]string, 0, len(files))
	for i := len(files) - 1; i >= 0; i-- {
		f = append(f, files[i])
	}
	runPool(b, f)
}

// The directory walk alone.
func BenchmarkWalk(b *testing.B) {
	root := os.Getenv("REPLAY_BENCH_CORPUS")
	if root == "" {
		b.Skip("no REPLAY_BENCH_CORPUS")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := transcriptFiles([]string{root}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBiggestFile parses the single largest transcript: the long pole.
func BenchmarkBiggestFile(b *testing.B) {
	files := benchCorpus(b)
	fi, _ := os.Stat(files[0])
	b.Logf("largest: %s (%d bytes)", files[0], fi.Size())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s, err := transcript.ParseClaudeCodeFile(files[0])
		if err != nil {
			b.Fatal(err)
		}
		lane := analysis.MainLane(s)
		_ = analysis.AnalyzeLane(s, lane)
	}
}
