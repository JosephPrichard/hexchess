package cmd

import (
	"bufio"
	"log/slog"
	"os"
	"strings"

	"hexchess-svc/pkg/logutil"
)

func InitEnv() {
	file, err := os.Open(".env")
	if err != nil {
		slog.Warn("did not load env file", "err", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		index := strings.Index(line, "=")
		if index < 0 {
			logutil.Fatal("invalid line in .env file", "line", line)
		}
		key, value := line[:index], line[index+1:]
		if err := os.Setenv(key, value); err != nil {
			slog.Warn("error setting env var", "err", err)
		}
	}
}
