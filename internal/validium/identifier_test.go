package validium

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEnvFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    map[string]string
		wantErr bool
	}{
		{
			name:    "simple key=value",
			content: "FOO=bar\nBAZ=qux\n",
			want:    map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:    "double-quoted values stripped",
			content: `DB_URL="postgres://localhost/db"`,
			want:    map[string]string{"DB_URL": "postgres://localhost/db"},
		},
		{
			name:    "single-quoted values stripped",
			content: "SECRET='mysecret'",
			want:    map[string]string{"SECRET": "mysecret"},
		},
		{
			name:    "comments and blank lines ignored",
			content: "# comment\n\nFOO=bar\n",
			want:    map[string]string{"FOO": "bar"},
		},
		{
			// Regression: the "#" check must run on the trimmed line, or an
			// indented comment containing "=" gets parsed as a variable.
			name:    "indented comment with = ignored",
			content: "  # FOO=bar\nREAL=value\n",
			want:    map[string]string{"REAL": "value"},
		},
		{
			name:    "lines without = ignored",
			content: "NOTAKEY\nFOO=bar\n",
			want:    map[string]string{"FOO": "bar"},
		},
		{
			name:    "export prefix stripped",
			content: "export FOO=bar\n",
			want:    map[string]string{"FOO": "bar"},
		},
		{
			name:    "value with = sign preserved",
			content: "TOKEN=abc=def=ghi",
			want:    map[string]string{"TOKEN": "abc=def=ghi"},
		},
		{
			name:    "empty file",
			content: "",
			want:    map[string]string{},
		},
		{
			name:    "mismatched quotes not stripped",
			content: `VAL="hello'`,
			want:    map[string]string{"VAL": `"hello'`},
		},
		{
			name:    "mismatched quotes reversed not stripped",
			content: `VAL='hello"`,
			want:    map[string]string{"VAL": `'hello"`},
		},
		{
			name:    "value containing internal quotes preserved",
			content: `VAL="it's fine"`,
			want:    map[string]string{"VAL": "it's fine"},
		},
		{
			name:    "long values above scanner default limit",
			content: "TOKEN=" + strings.Repeat("a", 70*1024),
			want:    map[string]string{"TOKEN": strings.Repeat("a", 70*1024)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ".env")
			require.NoError(t, os.WriteFile(path, []byte(tt.content), 0644))

			got, err := ParseEnvFile(path)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIdentifyVariables_TypeDetection(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		key      string
		wantType string
	}{
		{"boolean true", "FLAG=true", "FLAG", "boolean"},
		{"boolean false", "FLAG=false", "FLAG", "boolean"},
		{"integer", "PORT=8080", "PORT", "integer"},
		{"float", "RATIO=1.5", "RATIO", "float"},
		{"http url", "API=https://example.com", "API", "url-http"},
		{"http plain url", "API=http://example.com", "API", "url-http"},
		{"embedded http url remains string", "MESSAGE=see https://example.com", "MESSAGE", "string"},
		{"generic url", "DSN=postgres://localhost/db", "DSN", "url"},
		{"path is string", "PATH_VAR=/some/path", "PATH_VAR", "string"},
		{"email", "OWNER=user@example.com", "OWNER", "email"},
		{"plain string", "NAME=hello", "NAME", "string"},

		// email heuristic edge cases
		{"email no dot in domain", "SCOPE=user@corp", "SCOPE", "string"},
		{"email at start", "WEIRD=@example.com", "WEIRD", "string"},
		{"email double at", "BAD=user@@example.com", "BAD", "string"},

		// numeric edge cases
		{"scientific notation is float", "VAL=1e5", "VAL", "float"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ".env")
			require.NoError(t, os.WriteFile(path, []byte(tt.content), 0644))

			vars, err := IdentifyVariables(path)
			require.NoError(t, err)
			require.Len(t, vars, 1)
			assert.Equal(t, tt.wantType, vars[0].Type)
		})
	}
}

func TestIsLikelyEmail(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"user@example.com", true},
		{"user@sub.example.com", true},
		{"user@corp", false},         // no dot in domain
		{"user@@example.com", false}, // double @
		{"@example.com", false},      // @ at start
		{"user@", false},             // nothing after @
		{"notanemail", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, isLikelyEmail(tt.input))
		})
	}
}
