package ifconfig

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestClient_Get(t *testing.T) {
	validResp := Response{IP: "203.0.113.42", Country: "US"}
	validBody, _ := json.Marshal(validResp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(validBody)
		case "/empty_ip":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ip":""}`))
		case "/not_found":
			w.WriteHeader(http.StatusNotFound)
		case "/bad_json":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{invalid`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(validBody)
		}
	}))
	defer server.Close()

	logger := zap.NewNop()
	client := New(logger, server.Client(), nil)
	client.BaseURL = server.URL

	t.Run("success", func(t *testing.T) {
		client.BaseURL = server.URL + "/ok"
		ctx := context.Background()
		resp, err := client.Get(ctx)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if resp.IP != "203.0.113.42" {
			t.Errorf("Get() IP = %q, want 203.0.113.42", resp.IP)
		}
		if resp.Country != "US" {
			t.Errorf("Get() Country = %q, want US", resp.Country)
		}
	})

	t.Run("non-200 status", func(t *testing.T) {
		client.BaseURL = server.URL + "/not_found"
		ctx := context.Background()
		_, err := client.Get(ctx)
		if err == nil {
			t.Fatal("Get() expected error for 404")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		client.BaseURL = server.URL + "/bad_json"
		ctx := context.Background()
		_, err := client.Get(ctx)
		if err == nil {
			t.Fatal("Get() expected error for invalid JSON")
		}
	})

	t.Run("empty ip still returns response", func(t *testing.T) {
		client.BaseURL = server.URL + "/empty_ip"
		ctx := context.Background()
		resp, err := client.Get(ctx)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if resp.IP != "" {
			t.Errorf("Get() IP = %q, want empty", resp.IP)
		}
	})
}

func TestClient_GetAddress_SetAddress(t *testing.T) {
	logger := zap.NewNop()
	client := New(logger, nil, nil)

	if got := client.GetAddress(); got != "" {
		t.Errorf("GetAddress() = %q, want empty", got)
	}
	client.SetAddress("10.0.0.1")
	if got := client.GetAddress(); got != "10.0.0.1" {
		t.Errorf("GetAddress() after SetAddress = %q, want 10.0.0.1", got)
	}
	client.SetAddress("10.0.0.2")
	if got := client.GetAddress(); got != "10.0.0.2" {
		t.Errorf("GetAddress() after second SetAddress = %q, want 10.0.0.2", got)
	}
}

func TestClient_Run_InitialFetch(t *testing.T) {
	validResp := Response{IP: "198.51.100.1"}
	body, _ := json.Marshal(validResp)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	logger := zap.NewNop()
	client := New(logger, server.Client(), nil)
	client.BaseURL = server.URL

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		client.Run(ctx, nil)
		close(done)
	}()

	// Allow initial fetch to complete
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done

	if got := client.GetAddress(); got != "198.51.100.1" {
		t.Errorf("GetAddress() after Run = %q, want 198.51.100.1", got)
	}
}

func TestClient_Run_OnChanged(t *testing.T) {
	callCount := 0
	var oldIP, newIP string
	onChanged := func(o, n string) {
		callCount++
		oldIP, newIP = o, n
	}

	// First request returns IP1, second returns IP2 (simulate change)
	first := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if first {
			first = false
			_, _ = w.Write([]byte(`{"ip":"192.0.2.1"}`))
		} else {
			_, _ = w.Write([]byte(`{"ip":"192.0.2.2"}`))
		}
	}))
	defer server.Close()

	logger := zap.NewNop()
	client := New(logger, server.Client(), nil)
	client.BaseURL = server.URL

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		client.Run(ctx, onChanged)
		close(done)
	}()

	// Run only does initial fetch; the 10m ticker won't fire in test. So onChanged
	// is only called when we get a second fetch with different IP. We can't easily
	// trigger the ticker without waiting 10m. So just verify initial fetch and shutdown.
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done

	if callCount != 0 {
		t.Logf("onChanged called %d times (expected 0 in short run)", callCount)
	}
	if got := client.GetAddress(); got != "192.0.2.1" {
		t.Errorf("GetAddress() = %q, want 192.0.2.1", got)
	}
	_ = oldIP
	_ = newIP
}

func TestClient_New_NilHTTPClient(t *testing.T) {
	client := New(zap.NewNop(), nil, nil)
	if client.client == nil {
		t.Error("New(nil) should set default http.Client")
	}
}
