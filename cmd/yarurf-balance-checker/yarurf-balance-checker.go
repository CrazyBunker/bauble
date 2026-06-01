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

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
	// Регистрация флага для пути к конфигу
	flag.StringVar(&configPath, "config", "config.yaml", "Path to config file")
	flag.Parse()

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
func (c *Config) IsNotifyTime() bool {
	now := time.Now()
	currentHour := now.Hour()
	currentMinute := now.Minute()

	// Проверяем совпадение часа
	if currentHour != c.Notify.Time.Hour {
		return false
	}

	// Проверяем, что находимся в пределах часа от указанного времени
	return currentMinute >= c.Notify.Time.Minute && currentMinute < c.Notify.Time.Minute+60
}

func printBalance(balance float64, config Config) {
	var template string
	daysLeft := int(balance / config.Cost)
	var urgency string
	if daysLeft <= 1 {
		urgency = "🔴 СРОЧНО"
	} else if daysLeft <= 3 {
		urgency = "🟠 СРОЧНО"
	} else {
		urgency = "🟡 ВНИМАНИЕ"
	}

	if daysLeft > 3 {
		template = config.Templates.HighBalance
	} else {
		template = config.Templates.LowBalance
		if config.IsNotifyTime() {
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
				config.Cost,
				daysLeft,
			)
			for _, chat := range config.Notify.TelegramChat {
				SendTelegramMessage(config.Notify.TelegramToken, chat, message, "")
			}
		}
	}
	fmt.Printf(template+"\n", balance)
}

func SendTelegramMessage(token string, chatID int64, message string, parseMode string) error {
	// Инициализация бота
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return err
	}

	// Создание конфигурации сообщения
	msg := tgbotapi.NewMessage(chatID, message)
	if parseMode != "" {
		msg.ParseMode = parseMode
	}

	// Отправка сообщения
	_, err = bot.Send(msg)
	return err
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
