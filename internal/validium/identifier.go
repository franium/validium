package validium

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

const maxEnvLineSize = 1024 * 1024

// stripQuotes removes a matched outer pair of double or single quotes.
// It only strips when both the first and last characters are the same quote type.
func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func ParseEnvFile(envFilename string) (map[string]string, error) {
	file, err := os.Open(envFilename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	result := make(map[string]string)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), maxEnvLineSize)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		// dotenv files commonly prefix assignments with "export" for shell sourcing.
		key = strings.TrimSpace(strings.TrimPrefix(key, "export "))
		value := stripQuotes(strings.TrimSpace(parts[1]))
		result[key] = value
	}
	return result, scanner.Err()
}

func IdentifyVariables(envFilename string) ([]Variable, error) {
	raw, err := ParseEnvFile(envFilename)
	if err != nil {
		return nil, err
	}

	results := make([]Variable, 0, len(raw))
	for key, value := range raw {
		results = append(results, Variable{Name: key, Type: inferType(value)})
	}
	return results, nil
}

func isInteger(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func isFloat(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

func isLikelyEmail(value string) bool {
	if strings.Count(value, "@") != 1 {
		return false
	}
	at := strings.Index(value, "@")
	return at > 0 && strings.Contains(value[at+1:], ".")
}
