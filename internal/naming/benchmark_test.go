// internal/naming/benchmark_test.go
package naming

import (
	"os"
	"strings"
	"testing"
)

// loadBenchmarkSamples loads samples from the benchmark files
func loadBenchmarkSamples(name string) []string {
	path := "testdata/benchmark/" + name + ".txt"
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	var samples []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			samples = append(samples, line)
		}
	}
	return samples
}

// BenchmarkParseMovie_Small benchmarks movie name parsing with 100 samples
func BenchmarkParseMovie_Small(b *testing.B) {
	samples := loadBenchmarkSamples("small_bench")
	if len(samples) == 0 {
		b.Skip("benchmark data not available")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseMovieName(samples[i%len(samples)])
	}
}

// BenchmarkParseMovie_Medium benchmarks movie name parsing with 1000 samples
func BenchmarkParseMovie_Medium(b *testing.B) {
	samples := loadBenchmarkSamples("medium_bench")
	if len(samples) == 0 {
		b.Skip("benchmark data not available")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseMovieName(samples[i%len(samples)])
	}
}

// BenchmarkParseMovie_Large benchmarks movie name parsing with 10000 samples
func BenchmarkParseMovie_Large(b *testing.B) {
	samples := loadBenchmarkSamples("large_bench")
	if len(samples) == 0 {
		b.Skip("benchmark data not available")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseMovieName(samples[i%len(samples)])
	}
}

// BenchmarkIsTVEpisodeFilename benchmarks TV episode detection
func BenchmarkIsTVEpisodeFilename(b *testing.B) {
	samples := loadBenchmarkSamples("medium_bench")
	if len(samples) == 0 {
		b.Skip("benchmark data not available")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsTVEpisodeFilename(samples[i%len(samples)])
	}
}

// BenchmarkStripReleaseMarkers benchmarks release marker stripping
func BenchmarkStripReleaseMarkers(b *testing.B) {
	samples := loadBenchmarkSamples("medium_bench")
	if len(samples) == 0 {
		b.Skip("benchmark data not available")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stripReleaseMarkers(samples[i%len(samples)])
	}
}
