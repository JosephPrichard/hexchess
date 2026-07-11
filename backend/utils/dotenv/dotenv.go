package dotenv

import (
	"bufio"
	"hexchess-svc/utils/logutil"
	"log/slog"
	"os"
	"strings"
)

func Load() {
	file, err := os.Open(".env")
	if err != nil {
		slog.Warn("did not load env file", "error", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		index := strings.Index(line, "=")
		if index < 0 {
			logutil.Fatal("invalid line in .env file", nil, "line", line)
		}
		key, value := line[:index], line[index+1:]
		// these environment variables last until the end of this process
		if err := os.Setenv(key, value); err != nil {
			slog.Warn("error setting env var", "error", err)
		}
	}
}
