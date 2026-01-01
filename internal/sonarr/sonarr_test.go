package sonarr

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	t.Run("url from config", func(t *testing.T) {
		client := NewClient(Config{URL: "http://mysonarr:8989", APIKey: "test-key"})
		if client.baseURL != "http://mysonarr:8989" {
			t.Errorf("expected configured URL, got %s", client.baseURL)
		}
		if client.apiKey != "test-key" {
			t.Errorf("expected api key test-key, got %s", client.apiKey)
		}
	})

	t.Run("custom timeout", func(t *testing.T) {
		client := NewClient(Config{
			URL:     "http://custom:9999",
			APIKey:  "custom-key",
			Timeout: 60 * time.Second,
		})
		if client.baseURL != "http://custom:9999" {
			t.Errorf("expected custom URL, got %s", client.baseURL)
		}
	})
}

func TestClientPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/api/v3/system/status" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SystemStatus{
			AppName: "Sonarr",
			Version: "4.0.0",
		})
	}))
	defer server.Close()

	client := NewClient(Config{URL: server.URL, APIKey: "test-key"})
	if err := client.Ping(); err != nil {
		t.Errorf("ping failed: %v", err)
	}
}

func TestClientPingUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(Config{URL: server.URL, APIKey: "wrong-key"})
	if err := client.Ping(); err == nil {
		t.Error("expected error for unauthorized request")
	}
}

func TestGetQueue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v3/queue") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		response := QueueResponse{
			Page:         1,
			PageSize:     25,
			TotalRecords: 2,
			Records: []QueueItem{
				{ID: 1, Title: "Show.S01E01", Status: "downloading"},
				{ID: 2, Title: "Show.S01E02", Status: "completed"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{URL: server.URL, APIKey: "test"})
	queue, err := client.GetQueue(1, 25)
	if err != nil {
		t.Fatalf("GetQueue failed: %v", err)
	}
	if len(queue.Records) != 2 {
		t.Errorf("expected 2 records, got %d", len(queue.Records))
	}
	if queue.Records[0].Title != "Show.S01E01" {
		t.Errorf("expected first record title Show.S01E01, got %s", queue.Records[0].Title)
	}
}

func TestGetStuckItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := QueueResponse{
			Page:         1,
			PageSize:     100,
			TotalRecords: 3,
			Records: []QueueItem{
				{ID: 1, Title: "Good.S01E01", TrackedDownloadStatus: "ok"},
				{ID: 2, Title: "Stuck.S01E01", TrackedDownloadStatus: "warning"},
				{ID: 3, Title: "Error.S01E01", TrackedDownloadStatus: "error"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{URL: server.URL, APIKey: "test"})
	stuck, err := client.GetStuckItems()
	if err != nil {
		t.Fatalf("GetStuckItems failed: %v", err)
	}
	if len(stuck) != 2 {
		t.Errorf("expected 2 stuck items, got %d", len(stuck))
	}
}

func TestGetAllSeries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/series" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		series := []Series{
			{ID: 1, Title: "Silo", Year: 2023, TvdbID: 123456},
			{ID: 2, Title: "Breaking Bad", Year: 2008, TvdbID: 789012},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(series)
	}))
	defer server.Close()

	client := NewClient(Config{URL: server.URL, APIKey: "test"})
	series, err := client.GetAllSeries()
	if err != nil {
		t.Fatalf("GetAllSeries failed: %v", err)
	}
	if len(series) != 2 {
		t.Errorf("expected 2 series, got %d", len(series))
	}
	if series[0].Title != "Silo" {
		t.Errorf("expected first series Silo, got %s", series[0].Title)
	}
}

func TestFindSeriesByTitle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		series := []Series{
			{ID: 1, Title: "Silo", TitleSlug: "silo"},
			{ID: 2, Title: "Breaking Bad", TitleSlug: "breaking-bad"},
			{ID: 3, Title: "Silicon Valley", TitleSlug: "silicon-valley"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(series)
	}))
	defer server.Close()

	client := NewClient(Config{URL: server.URL, APIKey: "test"})

	matches, err := client.FindSeriesByTitle("sil")
	if err != nil {
		t.Fatalf("FindSeriesByTitle failed: %v", err)
	}
	if len(matches) != 2 {
		t.Errorf("expected 2 matches for 'sil', got %d", len(matches))
	}
}

func TestExecuteCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/api/v3/command" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var cmd Command
		json.NewDecoder(r.Body).Decode(&cmd)

		response := CommandResponse{
			ID:          1,
			Name:        cmd.Name,
			CommandName: cmd.Name,
			Status:      "queued",
			Message:     "Command queued",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{URL: server.URL, APIKey: "test"})
	resp, err := client.TriggerDownloadedEpisodesScan("/path/to/download")
	if err != nil {
		t.Fatalf("TriggerDownloadedEpisodesScan failed: %v", err)
	}
	if resp.Name != "DownloadedEpisodesScan" {
		t.Errorf("expected command name DownloadedEpisodesScan, got %s", resp.Name)
	}
	if resp.Status != "queued" {
		t.Errorf("expected status queued, got %s", resp.Status)
	}
}

func TestRemoveFromQueue(t *testing.T) {
	deleteCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		deleteCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{URL: server.URL, APIKey: "test"})
	err := client.RemoveFromQueue(123, false, false)
	if err != nil {
		t.Fatalf("RemoveFromQueue failed: %v", err)
	}
	if !deleteCalled {
		t.Error("DELETE was not called")
	}
}

func TestGetCommandStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/command/123" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		response := CommandResponse{
			ID:     123,
			Name:   "DownloadedEpisodesScan",
			Status: "completed",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{URL: server.URL, APIKey: "test"})
	status, err := client.GetCommandStatus(123)
	if err != nil {
		t.Fatalf("GetCommandStatus failed: %v", err)
	}
	if status.Status != "completed" {
		t.Errorf("expected status completed, got %s", status.Status)
	}
}
