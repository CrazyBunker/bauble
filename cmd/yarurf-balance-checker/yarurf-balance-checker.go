package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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

	Trigger float64 `yaml:"trigger"`
}

type AuthRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type BalanceResponse struct {
	Money float64 `json:"money"`
}

var (
	client     *http.Client
	configPath string // Переменная для хранения пути к конфигу
)

func init() {
	// Инициализация HTTP клиента
	jar, _ := cookiejar.New(nil)
	client = &http.Client{
		Jar:     jar,
		Timeout: 10 * time.Second,
	}

	// Регистрация флага для пути к конфигу
	flag.StringVar(&configPath, "config", "config.yaml", "Path to config file")
	flag.Parse()
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

func auth(config *Config) error {
	authReq := AuthRequest{
		Login:    config.User.Login,
		Password: config.User.Password,
	}

	jsonData, _ := json.Marshal(authReq)
	req, _ := http.NewRequest("POST", "https://yarurf.ru/api/lk/auth", bytes.NewBuffer(jsonData))

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

func getBalance() (*BalanceResponse, error) {
	req, _ := http.NewRequest("GET", "https://yarurf.ru/api/lk/get_base_info", nil)

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("balance request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var balance BalanceResponse
	if err := json.Unmarshal(body, &balance); err != nil {
		return nil, fmt.Errorf("failed to parse balance: %w", err)
	}

	return &balance, nil
}

func printBalance(balance float64, config Config) {
	var template string

	if balance > config.Trigger {
		template = config.Templates.HighBalance
	} else {
		template = config.Templates.LowBalance
	}

	fmt.Printf(template+"\n", balance)
}

func main() {
	// Загрузка конфига с учетом флага
	config, err := loadConfig(configPath)
	if err != nil {
		fmt.Printf("Config error: %v\n", err)
		os.Exit(1)
	}

	// Авторизация
	if err := auth(config); err != nil {
		fmt.Printf("Auth error: %v\n", err)
		os.Exit(1)
	}

	// Получение баланса
	balance, err := getBalance()
	if err != nil {
		fmt.Printf("Balance error: %v\n", err)
		os.Exit(1)
	}

	// Вывод результата
	printBalance(balance.Money, *config)
}
