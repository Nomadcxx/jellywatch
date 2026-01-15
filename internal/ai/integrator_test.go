package ai

import (
	"sync"
	"testing"
)

func TestNewIntegrator_Disabled(t *testing.T) {
	cfg := Config{Enabled: false}

	integrator, err := NewIntegrator(cfg, nil)
	if err != nil {
		t.Fatalf("NewIntegrator() error = %v", err)
	}

	if integrator.enabled {
		t.Error("Integrator should be disabled")
	}

	if integrator.regexConfidence == nil {
		t.Error("Disabled integrator should still have confidence calculator")
	}
}

func TestNewIntegrator_Enabled_NoMatcher(t *testing.T) {
	cfg := Config{
		Enabled:        true,
		OllamaEndpoint: "http://invalid:11434", // Will fail to connect
		Model:          "test",
	}

	// Should degrade gracefully to disabled
	integrator, err := NewIntegrator(cfg, nil)
	if err != nil {
		t.Fatalf("NewIntegrator() should not error, got: %v", err)
	}

	// Should have degraded to disabled
	if integrator.enabled {
		t.Error("Integrator should have degraded to disabled when matcher fails")
	}
}

func TestIntegrator_Close(t *testing.T) {
	cfg := Config{Enabled: false}
	integrator, _ := NewIntegrator(cfg, nil)

	// Should not panic or block
	err := integrator.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestIntegrator_EnhanceTitle_Disabled(t *testing.T) {
	cfg := Config{Enabled: false}
	integrator, _ := NewIntegrator(cfg, nil)

	// Caller does regex parsing first
	regexTitle := "Breaking Bad"
	filename := "Breaking.Bad.S01E01.1080p.mkv"

	title, source, err := integrator.EnhanceTitle(regexTitle, filename, "tv")
	if err != nil {
		t.Fatalf("EnhanceTitle() error = %v", err)
	}

	if source != SourceRegex {
		t.Errorf("source = %v, want SourceRegex", source)
	}

	if title == "" {
		t.Error("title should not be empty")
	}

	if title != regexTitle {
		t.Errorf("title = %q, want %q (regexTitle should be returned when AI disabled)", title, regexTitle)
	}
}

func TestIntegrator_EnhanceTitle_Concurrent(t *testing.T) {
	cfg := Config{Enabled: false}
	integrator, _ := NewIntegrator(cfg, nil)

	// Run 100 concurrent parses of the same filename
	var wg sync.WaitGroup
	filename := "Breaking.Bad.S01E01.1080p.mkv"
	regexTitle := "Breaking Bad"
	results := make(chan string, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			title, _, _ := integrator.EnhanceTitle(regexTitle, filename, "tv")
			results <- title
		}()
	}

	wg.Wait()
	close(results)

	// All results should be identical (singleflight guarantees this)
	var first string
	for title := range results {
		if first == "" {
			first = title
		} else if title != first {
			t.Errorf("Inconsistent results: got %q and %q", first, title)
		}
	}
}

func BenchmarkIntegrator_AIDisabled(b *testing.B) {
	cfg := Config{Enabled: false}
	integrator, _ := NewIntegrator(cfg, nil)
	filename := "Test.Show.S01E01.1080p.BluRay.x264-GROUP.mkv"
	regexTitle := "Test Show"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		integrator.EnhanceTitle(regexTitle, filename, "tv")
	}
	// Target: < 100μs (< 100,000 ns/op)
}
