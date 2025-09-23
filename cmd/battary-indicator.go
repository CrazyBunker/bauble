package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// BatteryInfo структура для хранения информации о батарее
type BatteryInfo struct {
	Level      int    // уровень заряда в процентах
	Status     string // статус зарядки
	ACPowered  bool   // подключен ли кабель питания
	USBPowered bool   // подключен ли USB
}

// Цветовые коды ANSI для bash
const (
	ColorDarkGreen  = "\033[32m" // темно-зеленый
	ColorLightGreen = "\033[92m" // светло-зеленый
	ColorYellow     = "\033[33m" // желтый
	ColorRed        = "\033[31m" // красный
	ColorReset      = "\033[0m"  // сброс цвета
)

// Цветовые коды в формате HEX для genmon
const (
	ColorGenmonDarkGreen  = "#006400" // темно-зеленый
	ColorGenmonLightGreen = "#32CD32" // светло-зеленый
	ColorGenmonYellow     = "#FFD700" // желтый
	ColorGenmonRed        = "#FF0000" // красный
)

// parseBatteryData парсит вывод команды dumpsys battery
func parseBatteryData(output string) (*BatteryInfo, error) {
	info := &BatteryInfo{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		switch {
		case strings.Contains(line, "level:"):
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				level, err := strconv.Atoi(parts[1])
				if err != nil {
					return nil, fmt.Errorf("ошибка парсинга уровня заряда: %v", err)
				}
				info.Level = level
			}

		case strings.Contains(line, "AC powered:"):
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				info.ACPowered = parts[1] == "true"
			}

		case strings.Contains(line, "USB powered:"):
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				info.USBPowered = parts[1] == "true"
			}

		case strings.Contains(line, "status:"):
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				statusCode, err := strconv.Atoi(parts[1])
				if err == nil {
					switch statusCode {
					case 2:
						info.Status = "Charging"
					case 3:
						info.Status = "Discharging"
					case 4:
						info.Status = "Not charging"
					case 5:
						info.Status = "Full"
					default:
						info.Status = "Unknown"
					}
				}
			}
		}
	}

	return info, nil
}

// getBatteryColor возвращает цветовой код на основе уровня заряда и статуса
func getBatteryColor(info *BatteryInfo, outputMode string) string {
	// Если устройство заряжается - темно-зеленый
	if info.Status == "Charging" || info.ACPowered || info.USBPowered {
		if outputMode == "genmon" {
			return ColorGenmonDarkGreen
		}
		return ColorDarkGreen
	}

	// Если разряжается - цвет зависит от уровня заряда
	switch {
	case info.Level >= 80:
		if outputMode == "genmon" {
			return ColorGenmonLightGreen
		}
		return ColorLightGreen
	case info.Level >= 20:
		if outputMode == "genmon" {
			return ColorGenmonYellow
		}
		return ColorYellow
	default:
		if outputMode == "genmon" {
			return ColorGenmonRed
		}
		return ColorRed
	}
}

// getBatteryData выполняет ADB-команду и возвращает структуру с данными
func getBatteryData() (*BatteryInfo, error) {
	cmd := exec.Command("adb", "shell", "dumpsys", "battery")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения ADB-команды: %v", err)
	}

	return parseBatteryData(string(output))
}

// outputBash выводит данные в формате для bash с ANSI цветами
func outputBash(info *BatteryInfo) {
	color := getBatteryColor(info, "bash")

	chargingIndicator := ""
	if info.Status == "Charging" || info.ACPowered || info.USBPowered {
		chargingIndicator = "⚡"
	}

	// Выводим результат с цветом
	fmt.Printf("%sУровень заряда: %d%% %s(%s)%s\n",
		color,
		info.Level,
		chargingIndicator,
		info.Status,
		ColorReset)

	// Дополнительная информация
	fmt.Printf("AC powered: %v\n", info.ACPowered)
	fmt.Printf("USB powered: %v\n", info.USBPowered)
}

// outputGenmon выводит данные в формате для Xfce genmon плагина
func outputGenmon(info *BatteryInfo) {
	color := getBatteryColor(info, "genmon")

	// Иконка в зависимости от статуса
	icon := "🔋" // батарея по умолчанию
	if info.Status == "Charging" || info.ACPowered || info.USBPowered {
		icon = "⚡" // молния при зарядке
	} else if info.Level < 20 {
		icon = "🪫" // разряженная батарея
	}

	// Основной текст для панели (краткий)
	fmt.Printf("<txt><span foreground='%s'>%s %d%%</span></txt>\n",
		color, icon, info.Level)

	// Подсказка при наведении (подробная информация)
	fmt.Printf("<tool><b>Статус батареи Android</b>\n")
	fmt.Printf("Уровень: %d%%\n", info.Level)
	fmt.Printf("Статус: %s\n", info.Status)
	fmt.Printf("AC питание: %v\n", info.ACPowered)
	fmt.Printf("USB питание: %v</tool>\n", info.USBPowered)
}

func main() {
	// Парсим аргументы командной строки
	outputFlag := flag.String("o", "genmon", "Режим вывода: genmon или bash")
	helpFlag := flag.Bool("h", false, "Показать справку")

	flag.Parse()

	if *helpFlag {
		fmt.Println("Использование: adb-battery [-o genmon|bash] [-h]")
		fmt.Println("  -o genmon   Вывод в формате Xfce genmon (по умолчанию)")
		fmt.Println("  -o bash     Вывод в формате для терминала с цветами")
		fmt.Println("  -h          Показать эту справку")
		os.Exit(0)
	}

	// Проверяем допустимые значения для -o
	if *outputFlag != "genmon" && *outputFlag != "bash" {
		fmt.Printf("Ошибка: неверное значение для -o: %s\n", *outputFlag)
		fmt.Println("Допустимые значения: genmon, bash")
		os.Exit(1)
	}

	// Получаем данные о батарее
	info, err := getBatteryData()
	if err != nil {
		if *outputFlag == "genmon" {
			fmt.Printf("<txt><span foreground='%s'>🔴 Ошибка</span></txt>\n", ColorGenmonRed)
			fmt.Printf("<tool>%s</tool>\n", err.Error())
		} else {
			fmt.Printf("Ошибка: %v\n", err)
		}
		os.Exit(1)
	}

	// Выводим в зависимости от выбранного режима
	if *outputFlag == "genmon" {
		outputGenmon(info)
	} else {
		outputBash(info)
	}
}
