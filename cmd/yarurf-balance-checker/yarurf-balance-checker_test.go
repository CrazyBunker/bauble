package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"
)

// helper to create temporary config file
func writeTempConfig(t *testing.T, content string) string {
	tmp, err := ioutil.TempFile("", "config-*.yaml")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	if _, err := tmp.WriteString(content); err != nil {
		t.Fatalf("write: %v", err)
	}
	tmp.Close()
	return tmp.Name()
}

func TestLoadConfig_Success(t *testing.T) {
	yaml := `
user:
  login: test
  password: pass
templates:
  high_balance: "High"
  low_balance: "Low"
cost: 10
notify:
  telegram_token: "token"
  chat: [12345]
  time:
    hour: 12
    minute: 30
`
	path := writeTempConfig(t, yaml)
	defer os.Remove(path)

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.User.Login != "test" {
		t.Errorf("expected login test, got %s", cfg.User.Login)
	}
}

func TestIsNotifyTime_MinuteBoundary(t *testing.T) {
	cfg := &Config{}
	cfg.Notify.Time.Hour = 10
	cfg.Notify.Time.Minute = 20
	nowFunc = func() time.Time { return time.Date(2026, 6, 7, 10, 75, 0, 0, time.UTC) }
	if cfg.IsNotifyTime() {
		t.Error("expected false when minute equals minute+60")
	}
}

func TestWaitForInternetConnection_ZeroRetries(t *testing.T) {
	attempts := 0
	checkInternet = func() bool {
		attempts++
		return true
	}
	ok := waitForInternetConnection(0, time.Millisecond)
	if ok {
		t.Error("expected false with zero retries")
	}
	if attempts != 0 {
		t.Errorf("expected 0 attempts, got %d", attempts)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := loadConfig("nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestCheckInternetConnection(t *testing.T) {
	_ = checkInternetConnection()
}

func TestWaitForInternetConnection(t *testing.T) {
	attempts := 0
	checkInternet = func() bool {
		attempts++
		return attempts >= 2
	}
	ok := waitForInternetConnection(5, time.Millisecond)
	if !ok {
		t.Error("expected true after retries")
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestAuth_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req AuthRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Login != "u" || req.Password != "p" {
			t.Errorf("wrong creds: %+v", req)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	proxyURL, _ := url.Parse(srv.URL)
	client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	var cfg *Config = &Config{}
	cfg.APIURL = srv.URL
	cfg.User.Login = "u"
	cfg.User.Password = "p"
	if err := auth(cfg); err != nil {
		t.Fatalf("auth failed: %v", err)
	}
}

func TestAuth_FailureStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	proxyURL, _ := url.Parse(srv.URL)
	client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	var cfg *Config = &Config{}
	cfg.APIURL = srv.URL
	cfg.User.Login = "u"
	cfg.User.Password = "p"
	if err := auth(cfg); err == nil {
		t.Error("expected error for non-200 status")
	}
}

func TestGetBalance_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BalanceResponse{Money: 123.45})
	}))
	defer srv.Close()

	proxyURL, _ := url.Parse(srv.URL)
	client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	var cfg *Config = &Config{}
	cfg.APIURL = srv.URL
	bal, err := getBalance(cfg)
	if err != nil {
		t.Fatalf("getBalance: %v", err)
	}
	if bal.Money != 123.45 {
		t.Errorf("expected 123.45, got %f", bal.Money)
	}
}

func TestGetBalance_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	proxyURL, _ := url.Parse(srv.URL)
	client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	var cfg *Config = &Config{}
	cfg.APIURL = srv.URL
	if _, err := getBalance(cfg); err == nil {
		t.Error("expected error on bad status")
	}
}

func TestIsNotifyTime(t *testing.T) {
	cfg := &Config{}
	cfg.Notify.Time.Hour = 10
	cfg.Notify.Time.Minute = 15
	nowFunc = func() time.Time { return time.Date(2026, 6, 7, 10, 15, 0, 0, time.UTC) }
	if !cfg.IsNotifyTime() {
		t.Error("expected true at matching time")
	}
	nowFunc = func() time.Time { return time.Date(2026, 6, 7, 9, 50, 0, 0, time.UTC) }
	if cfg.IsNotifyTime() {
		t.Error("expected false when hour mismatched")
	}
}

func TestPrintBalance_NoNotify(t *testing.T) {
	t.Skip("Skipping printBalance test due to SendTelegramMessage dependency")
}

func TestSendTelegramMessage(t *testing.T) {
	t.Skip("Skipping SendTelegramMessage test due to external dependency")
}

// New test for printBalance output
func captureOutput(f func()) string {
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = origStdout
	return buf.String()
}

func TestPrintBalance_Output(t *testing.T) {
	cfgHigh := &Config{}
	cfgHigh.Notify.Time.Hour = 0
	cfgHigh.Notify.Time.Minute = 0
	cfgHigh.Templates.HighBalance = "HIGH"
	cfgHigh.Templates.LowBalance = "LOW"
	cfgHigh.Cost = 1
	balanceHigh := 10.0 // daysLeft = 10 > 3
	outHigh := captureOutput(func() {
		printBalance(balanceHigh, *cfgHigh)
	})
	if !bytes.Contains([]byte(outHigh), []byte("HIGH")) {
		t.Errorf("expected high balance template in output, got %q", outHigh)
	}

	cfgLow := &Config{}
	cfgLow.Notify.Time.Hour = 0
	cfgLow.Notify.Time.Minute = 0
	cfgLow.Templates.HighBalance = "HIGH"
	cfgLow.Templates.LowBalance = "LOW"
	cfgLow.Cost = 1
	balanceLow := 2.0 // daysLeft = 2 <= 3
	outLow := captureOutput(func() {
		printBalance(balanceLow, *cfgLow)
	})
	if !bytes.Contains([]byte(outLow), []byte("LOW")) {
		t.Errorf("expected low balance template in output, got %q", outLow)
	}
}
