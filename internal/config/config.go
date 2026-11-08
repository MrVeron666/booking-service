// Пакет config читает настройки приложения из переменных окружения.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Config хранит всё, что нужно сервису для запуска.
type Config struct {
	HTTPAddr    string // адрес HTTP-сервера, например :8080
	DatabaseURL string // строка подключения к PostgreSQL
}

// Load сначала подхватывает файл .env (если он есть), затем читает переменные окружения.
func Load() (Config, error) {
	err := loadDotEnv(".env")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTPAddr:    os.Getenv("HTTP_ADDR"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}

	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("не задана переменная DATABASE_URL")
	}

	return cfg, nil
}

// loadDotEnv читает пары KEY=VALUE из файла. Если файла нет — это нормально
// (в Docker переменные уже переданы через environment).
func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("не удалось открыть %s: %w", path, err)
	}
	defer func() {
		closeErr := file.Close()
		if closeErr != nil {
			fmt.Fprintf(os.Stderr, "не удалось закрыть %s: %v\n", path, closeErr)
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("некорректная строка в %s: %q", path, line)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// Не перезаписываем то, что уже задано в окружении (удобно для Docker).
		_, alreadySet := os.LookupEnv(key)
		if alreadySet {
			continue
		}

		err = os.Setenv(key, value)
		if err != nil {
			return fmt.Errorf("не удалось задать %s: %w", key, err)
		}
	}

	err = scanner.Err()
	if err != nil {
		return fmt.Errorf("ошибка чтения %s: %w", path, err)
	}

	return nil
}
