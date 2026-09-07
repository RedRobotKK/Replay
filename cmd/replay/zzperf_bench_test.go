package main

// SCRATCH. Delete before finishing.

import (
	"io"
	"os"
	"testing"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

func corpusFiles(b *testing.B) []string {
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

// Stage 1: read the bytes only. The I/O floor.
func BenchmarkStage1ReadBytes(b *testing.B) {
	files := corpusFiles(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var n int64
		for _, f := range files {
			fh, err := os.Open(f)
			if err != nil {
				continue
			}
			c, _ := io.Copy(io.Discard, fh)
			n += c
			fh.Close()
		}
		b.SetBytes(n / int64(max(1, b.N)))
	}
}

// Stage 2: read + split lines. Scanner cost, no JSON.
func BenchmarkStage2ScanLines(b *testing.B) {
	files := corpusFiles(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, f := range files {
			fh, err := os.Open(f)
			if err != nil {
				continue
			}
			sc := transcript.NewLineScanner(fh)
			for sc.Scan() {
				_ = sc.Bytes()
			}
			fh.Close()
		}
	}
}

// Stage 3: full parse into transcript.Session.
func BenchmarkStage3Parse(b *testing.B) {
	files := corpusFiles(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, f := range files {
			s, err := transcript.ParseClaudeCodeFile(f)
			if err != nil {
				continue
			}
			_ = s
		}
	}
}

// Stage 4: parse + MainLane + AnalyzeLane, serially. What cost does per file.
func BenchmarkStage4ParseAnalyze(b *testing.B) {
	files := corpusFiles(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, f := range files {
			s, err := loadSession(f)
			if err != nil {
				continue
			}
			lane := analysis.MainLane(s)
			if lane == nil {
				continue
			}
			_ = analysis.AnalyzeLane(s, lane)
		}
	}
}

// Stage 5: the real cold path, through the worker pool.
func BenchmarkStage5ForEachSession(b *testing.B) {
	files := corpusFiles(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = forEachSession(files, func(string, *transcript.Session, *analysis.LaneReport, error) error {
			return nil
		})
	}
}
