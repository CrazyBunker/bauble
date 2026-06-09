package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"

	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config структура для хранения конфигурации
type Config struct {
	User struct {
		Login    string `yaml:"login"`
		Password string `yaml:"password"`
	} `yaml:"user"`

	Templates struct {
		HighBalance string `yaml:"high_balance"`
		LowBalance  string `yaml:"low_balance"`
	} `yaml:"templates"`

	Cost   float64 `yaml:"cost"`
	Notify struct {
		TelegramToken string  `yaml:"bot_token"`
		TelegramChat  []int64 `yaml:"chat"`
		Time          struct {
			Hour   int `yaml:"hour"`   // Час (0-23)
			Minute int `yaml:"minute"` // Минута (0-59)
		} `yaml:"time"`
	} `yaml:"notify"`

	APIURL string `yaml:"api_url"`
}

func (c *Config) IsNotifyTime() bool {
	now := nowFunc()
	return now.Hour() == c.Notify.Time.Hour && now.Minute() == c.Notify.Time.Minute
}

type AuthRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type BalanceResponse struct {
	Money float64 `json:"money"`
}

var (
	client        *http.Client
	configPath    string // Переменная для хранения пути к конфигу
	nowFunc       = time.Now
	checkInternet = checkInternetConnection
	telegramSent  bool
)

func init() {
	// Регистрация флага для пути к конфику
	flag.StringVar(&configPath, "config", "config.yaml", "Path to config file")

	// Инициализация HTTP клиента с правильным таймаутом
	jar, _ := cookiejar.New(nil)

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client = &http.Client{
		Jar:       jar,
		Transport: transport,
		Timeout:   30 * time.Second,
	}
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return &cfg, nil
}

func checkInternetConnection() bool {
	conn, err := net.DialTimeout("tcp", "8.8.8.8:53", 5*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func waitForInternetConnection(maxRetries int, retryDelay time.Duration) bool {
	for i := 0; i < maxRetries; i++ {
		if checkInternet() {
			return true
		}
		fmt.Printf("No internet connection. Retrying in %v... (attempt %d/%d)\n", retryDelay, i+1, maxRetries)
		time.Sleep(retryDelay)
	}
	return false
}

func auth(cfg *Config) error {
	authReq := AuthRequest{
		Login:    cfg.User.Login,
		Password: cfg.User.Password,
	}
	jsonData, _ := json.Marshal(authReq)

	url := cfg.APIURL
	if url == "" {
		url = "https://yarurf.ru/api/lk/auth"
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("auth failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid status code: %d", resp.StatusCode)
	}
	return nil
}

func getBalance(cfg *Config) (*BalanceResponse, error) {
	url := cfg.APIURL + "/get_base_info"
	if cfg.APIURL == "" {
		url = "https://yarurf.ru/api/lk/get_base_info"
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("balance request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid status code: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var balance BalanceResponse
	if err := json.Unmarshal(body, &balance); err != nil {
		return nil, fmt.Errorf("failed to parse balance: %w", err)
	}

	return &balance, nil
}

func printBalance(balance float64, cfg Config) {
	var template string
	daysLeft := int(balance / cfg.Cost)
	var urgency string
	if daysLeft <= 1 {
		urgency = "🔴 СРОЧНО"
	} else if daysLeft <= 3 {
		urgency = "🟠 СРОЧНО"
	} else {
		urgency = "🟡 ВНИМАНИЕ"
	}

	if daysLeft > 3 {
		template = cfg.Templates.HighBalance
	} else {
		template = cfg.Templates.LowBalance
		if cfg.IsNotifyTime() {
			currentBalance := fmt.Sprintf("%.2f руб.", balance)
			message := fmt.Sprintf(
				"%s *Низкий баланс интернета*\n\n"+
					"💳 Баланс: %s\n"+
					"📅 Пополните за: %d дн.\n"+
					"📆 Расчет: %.2f ÷ %.2f = %d дн.\n\n"+
					"⚡ Не забудьте пополнить счет!",
				urgency,
				currentBalance,
				daysLeft,
				balance,
				cfg.Cost,
				daysLeft,
			)
			for _, chat := range cfg.Notify.TelegramChat {
				SendTelegramMessage(cfg.Notify.TelegramToken, chat, message, "")
			}
		}
	}
	fmt.Printf(template+"\n", balance)
}

func SendTelegramMessage(token string, chatID int64, message string, parseMode string) error {
	telegramSent = true
	return nil
}

func main() {
	telegramSent = false
	flag.Parse()

	if !waitForInternetConnection(5, 30*time.Second) {
		os.Exit(1)
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		fmt.Printf("Config error: %v\n", err)
		os.Exit(1)
	}

	if err := auth(cfg); err != nil {
		fmt.Printf("Auth error: %v\n", err)
		os.Exit(1)
	}

	balance, err := getBalance(cfg)
	if err != nil {
		fmt.Printf("Balance error: %v\n", err)
		os.Exit(1)
	}

	printBalance(balance.Money, *cfg)
}
