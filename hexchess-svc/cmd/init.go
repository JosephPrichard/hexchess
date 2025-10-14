package cmd

import (
	"bufio"
	"log"
	"os"
	"strings"
)

func InitEnv() {
	file, err := os.Open(".env")
	if err != nil {
		log.Printf("error loading .env file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		index := strings.Index(line, "=")
		if index < 0 {
			log.Fatalf("invalid line in .env file: %s", line)
		}
		key, value := line[:index], line[index+1:]
		if err := os.Setenv(key, value); err != nil {
			log.Printf("error setting env var: %v", err)
		}
	}
}
