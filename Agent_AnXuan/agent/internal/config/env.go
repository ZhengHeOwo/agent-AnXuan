package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func LoadEnvFile(fileName string) error {
	file, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("打开环境配置文件 %q 失败: %w", fileName, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++

		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("第 %d 行不是合法的 KEY=VALUE 格式", lineNumber)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" {
			return fmt.Errorf("第 %d 行的环境变量名称为空", lineNumber)
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("[环境配置] 第 %d 行失败:%w", lineNumber, err)
		}

	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("[环境配置|扫描器异常]:%w", err)
	}

	return nil
}
