package main

import (
	"bufio"
	"os"
	"strings"
)

func loadAPIKeyFromDotEnv(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key != "API_KEY" {
			continue
		}

		val = strings.Trim(val, "\"'")
		if val == "" {
			return "", nil
		}
		return val, nil
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", nil
}