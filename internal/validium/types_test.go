package validium

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidiumVariable_Validate(t *testing.T) {
	tests := []struct {
		name        string
		variable    ValidiumVariable
		actualValue string
		wantErr     bool
	}{
		// required
		{name: "required missing value", variable: ValidiumVariable{Name: "X", Type: "string", Required: true}, actualValue: "", wantErr: true},
		{name: "required present value", variable: ValidiumVariable{Name: "X", Type: "string", Required: true}, actualValue: "hello", wantErr: false},

		// string
		{name: "string valid", variable: ValidiumVariable{Name: "X", Type: "string"}, actualValue: "anything", wantErr: false},
		{name: "string empty optional", variable: ValidiumVariable{Name: "X", Type: "string"}, actualValue: "", wantErr: false},

		// string with choices condition
		{name: "string choices valid match", variable: ValidiumVariable{Name: "X", Type: "string", Conditions: json.RawMessage(`{"choices":["dev","staging","prod"]}`)}, actualValue: "prod", wantErr: false},
		{name: "string choices first element passes", variable: ValidiumVariable{Name: "X", Type: "string", Conditions: json.RawMessage(`{"choices":["dev","staging","prod"]}`)}, actualValue: "dev", wantErr: false},
		{name: "string choices not in list fails", variable: ValidiumVariable{Name: "X", Type: "string", Conditions: json.RawMessage(`{"choices":["dev","staging","prod"]}`)}, actualValue: "production", wantErr: true},
		{name: "string choices empty list allows any", variable: ValidiumVariable{Name: "X", Type: "string", Conditions: json.RawMessage(`{"choices":[]}`)}, actualValue: "anything", wantErr: false},
		{name: "string choices null conditions allows any", variable: ValidiumVariable{Name: "X", Type: "string", Conditions: json.RawMessage(`null`)}, actualValue: "anything", wantErr: false},
		{name: "string choices invalid json fails", variable: ValidiumVariable{Name: "X", Type: "string", Conditions: json.RawMessage(`{bad json}`)}, actualValue: "anything", wantErr: true},

		// optional empty values — format validation must be skipped
		{name: "url optional empty", variable: ValidiumVariable{Name: "X", Type: "url"}, actualValue: "", wantErr: false},
		{name: "url-http optional empty", variable: ValidiumVariable{Name: "X", Type: "url-http"}, actualValue: "", wantErr: false},
		{name: "email optional empty", variable: ValidiumVariable{Name: "X", Type: "email"}, actualValue: "", wantErr: false},
		{name: "integer optional empty", variable: ValidiumVariable{Name: "X", Type: "integer"}, actualValue: "", wantErr: false},
		{name: "float optional empty", variable: ValidiumVariable{Name: "X", Type: "float"}, actualValue: "", wantErr: false},
		{name: "boolean optional empty", variable: ValidiumVariable{Name: "X", Type: "boolean"}, actualValue: "", wantErr: false},
		// required empty must still fail
		{name: "url required empty", variable: ValidiumVariable{Name: "X", Type: "url", Required: true}, actualValue: "", wantErr: true},
		{name: "email required empty", variable: ValidiumVariable{Name: "X", Type: "email", Required: true}, actualValue: "", wantErr: true},

		// integer
		{name: "integer valid", variable: ValidiumVariable{Name: "X", Type: "integer"}, actualValue: "42", wantErr: false},
		{name: "integer invalid", variable: ValidiumVariable{Name: "X", Type: "integer"}, actualValue: "abc", wantErr: true},
		{name: "integer float rejected", variable: ValidiumVariable{Name: "X", Type: "integer"}, actualValue: "3.14", wantErr: true},

		// float
		{name: "float valid", variable: ValidiumVariable{Name: "X", Type: "float"}, actualValue: "3.14", wantErr: false},
		{name: "float integer accepted", variable: ValidiumVariable{Name: "X", Type: "float"}, actualValue: "42", wantErr: false},
		{name: "float invalid", variable: ValidiumVariable{Name: "X", Type: "float"}, actualValue: "abc", wantErr: true},

		// boolean
		{name: "boolean true", variable: ValidiumVariable{Name: "X", Type: "boolean"}, actualValue: "true", wantErr: false},
		{name: "boolean false", variable: ValidiumVariable{Name: "X", Type: "boolean"}, actualValue: "false", wantErr: false},
		{name: "boolean invalid", variable: ValidiumVariable{Name: "X", Type: "boolean"}, actualValue: "yes", wantErr: true},
		{name: "boolean 1 rejected", variable: ValidiumVariable{Name: "X", Type: "boolean"}, actualValue: "1", wantErr: true},

		// url
		{name: "url valid relative", variable: ValidiumVariable{Name: "X", Type: "url"}, actualValue: "/api/v1", wantErr: false},
		{name: "url valid postgres", variable: ValidiumVariable{Name: "X", Type: "url"}, actualValue: "postgres://localhost/db", wantErr: false},
		{name: "url invalid spaces", variable: ValidiumVariable{Name: "X", Type: "url"}, actualValue: "not a url", wantErr: true},
		{name: "url no scheme or slash", variable: ValidiumVariable{Name: "X", Type: "url"}, actualValue: "notaurl", wantErr: true},
		{name: "url bare domain rejected", variable: ValidiumVariable{Name: "X", Type: "url"}, actualValue: "example.com", wantErr: true},

		// url-http
		{name: "url-http valid https", variable: ValidiumVariable{Name: "X", Type: "url-http"}, actualValue: "https://example.com", wantErr: false},
		{name: "url-http valid http", variable: ValidiumVariable{Name: "X", Type: "url-http"}, actualValue: "http://example.com", wantErr: false},
		{name: "url-http no scheme", variable: ValidiumVariable{Name: "X", Type: "url-http"}, actualValue: "example.com", wantErr: true},
		{name: "url-http postgres rejected", variable: ValidiumVariable{Name: "X", Type: "url-http"}, actualValue: "postgres://localhost/db", wantErr: true},

		// email
		{name: "email valid", variable: ValidiumVariable{Name: "X", Type: "email"}, actualValue: "user@example.com", wantErr: false},
		{name: "email no at sign", variable: ValidiumVariable{Name: "X", Type: "email"}, actualValue: "notanemail", wantErr: true},
		{name: "email no domain", variable: ValidiumVariable{Name: "X", Type: "email"}, actualValue: "user@", wantErr: true},
		{name: "email display name rejected", variable: ValidiumVariable{Name: "X", Type: "email"}, actualValue: "John Doe <user@example.com>", wantErr: true},
		{name: "email angle brackets rejected", variable: ValidiumVariable{Name: "X", Type: "email"}, actualValue: "<user@example.com>", wantErr: true},

		// unknown type
		{name: "unknown type returns error", variable: ValidiumVariable{Name: "X", Type: "unknown"}, actualValue: "x", wantErr: true},

		// url-http with explicit conditions
		{name: "url-http https required accepts https", variable: ValidiumVariable{Name: "X", Type: "url-http", Conditions: json.RawMessage(`{"https":true}`)}, actualValue: "https://example.com", wantErr: false},
		{name: "url-http https required rejects http", variable: ValidiumVariable{Name: "X", Type: "url-http", Conditions: json.RawMessage(`{"https":true}`)}, actualValue: "http://example.com", wantErr: true},
		{name: "url-http https false allows http", variable: ValidiumVariable{Name: "X", Type: "url-http", Conditions: json.RawMessage(`{"https":false}`)}, actualValue: "http://example.com", wantErr: false},
		{name: "url-http null conditions allows http", variable: ValidiumVariable{Name: "X", Type: "url-http", Conditions: json.RawMessage(`null`)}, actualValue: "http://example.com", wantErr: false},
		{name: "url-http no conditions allows http", variable: ValidiumVariable{Name: "X", Type: "url-http"}, actualValue: "http://example.com", wantErr: false},

		// integer with min/max conditions
		{name: "integer no conditions passes any value", variable: ValidiumVariable{Name: "X", Type: "integer"}, actualValue: "999", wantErr: false},
		{name: "integer at min passes", variable: ValidiumVariable{Name: "X", Type: "integer", Conditions: json.RawMessage(`{"min":1}`)}, actualValue: "1", wantErr: false},
		{name: "integer above min passes", variable: ValidiumVariable{Name: "X", Type: "integer", Conditions: json.RawMessage(`{"min":1}`)}, actualValue: "5", wantErr: false},
		{name: "integer below min fails", variable: ValidiumVariable{Name: "X", Type: "integer", Conditions: json.RawMessage(`{"min":1}`)}, actualValue: "0", wantErr: true},
		{name: "integer at max passes", variable: ValidiumVariable{Name: "X", Type: "integer", Conditions: json.RawMessage(`{"max":100}`)}, actualValue: "100", wantErr: false},
		{name: "integer above max fails", variable: ValidiumVariable{Name: "X", Type: "integer", Conditions: json.RawMessage(`{"max":100}`)}, actualValue: "101", wantErr: true},
		{name: "integer within range passes", variable: ValidiumVariable{Name: "X", Type: "integer", Conditions: json.RawMessage(`{"min":1,"max":10}`)}, actualValue: "5", wantErr: false},
		{name: "integer below range fails", variable: ValidiumVariable{Name: "X", Type: "integer", Conditions: json.RawMessage(`{"min":1,"max":10}`)}, actualValue: "0", wantErr: true},
		{name: "integer above range fails", variable: ValidiumVariable{Name: "X", Type: "integer", Conditions: json.RawMessage(`{"min":1,"max":10}`)}, actualValue: "11", wantErr: true},
		{name: "integer null conditions passes", variable: ValidiumVariable{Name: "X", Type: "integer", Conditions: json.RawMessage(`null`)}, actualValue: "999", wantErr: false},

		// float with min/max conditions
		{name: "float no conditions passes any value", variable: ValidiumVariable{Name: "X", Type: "float"}, actualValue: "3.14", wantErr: false},
		{name: "float at min passes", variable: ValidiumVariable{Name: "X", Type: "float", Conditions: json.RawMessage(`{"min":0.0}`)}, actualValue: "0.0", wantErr: false},
		{name: "float above min passes", variable: ValidiumVariable{Name: "X", Type: "float", Conditions: json.RawMessage(`{"min":0.0}`)}, actualValue: "0.1", wantErr: false},
		{name: "float below min fails", variable: ValidiumVariable{Name: "X", Type: "float", Conditions: json.RawMessage(`{"min":0.0}`)}, actualValue: "-0.1", wantErr: true},
		{name: "float at max passes", variable: ValidiumVariable{Name: "X", Type: "float", Conditions: json.RawMessage(`{"max":1.0}`)}, actualValue: "1.0", wantErr: false},
		{name: "float above max fails", variable: ValidiumVariable{Name: "X", Type: "float", Conditions: json.RawMessage(`{"max":1.0}`)}, actualValue: "1.1", wantErr: true},
		{name: "float within range passes", variable: ValidiumVariable{Name: "X", Type: "float", Conditions: json.RawMessage(`{"min":0.0,"max":1.0}`)}, actualValue: "0.5", wantErr: false},
		{name: "float null conditions passes", variable: ValidiumVariable{Name: "X", Type: "float", Conditions: json.RawMessage(`null`)}, actualValue: "3.14", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.variable.Validate(tt.actualValue)
			if tt.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestParseValidiumData_RejectsUnknownType(t *testing.T) {
	data := ValidiumData{
		Variables: map[string]ValidiumVariable{
			"X": {Type: "badtype"},
		},
	}
	_, err := ParseValidiumData(data)
	assert.Error(t, err, "expected error for unknown type")
}

func TestParseValidiumData_SetsName(t *testing.T) {
	data := ValidiumData{
		Variables: map[string]ValidiumVariable{
			"MY_VAR": {Type: "string"},
		},
	}
	vars, err := ParseValidiumData(data)
	require.NoError(t, err)
	require.Len(t, vars, 1)
	assert.Equal(t, "MY_VAR", vars[0].Name)
}

func TestFormatValidiumData(t *testing.T) {
	vars := []ValidiumVariable{
		{Name: "NAME", Type: "string"},
		{Name: "PORT", Type: "integer"},
	}

	data := FormatValidiumData(vars)

	assert.NotEmpty(t, data.Version)
	require.Len(t, data.Variables, 2)

	name, ok := data.Variables["NAME"]
	require.True(t, ok, "NAME variable missing")
	assert.Equal(t, "string", name.Type)

	port, ok := data.Variables["PORT"]
	require.True(t, ok, "PORT variable missing")
	assert.Equal(t, "integer", port.Type)
}

func TestURLHTTPZeroConditions_NotSerialized(t *testing.T) {
	// A url-http variable with no explicit conditions (nil Conditions) must not
	// emit a "conditions" key in the generated JSON.
	source := []ValidiumVariable{
		{Name: "API", Type: "url-http"}, // Conditions is nil
	}
	data := FormatValidiumData(source)

	out, err := json.Marshal(data.Variables["API"])
	require.NoError(t, err)
	assert.NotContains(t, string(out), "conditions",
		"expected no conditions key for nil-condition url-http")
}

func TestStringChoicesConditions_RoundTrip(t *testing.T) {
	source := []ValidiumVariable{
		{Name: "ENV", Type: "string", Conditions: json.RawMessage(`{"choices":["dev","staging","prod"]}`)},
	}

	data := FormatValidiumData(source)
	vars, err := ParseValidiumData(data)
	require.NoError(t, err)
	require.Len(t, vars, 1)

	v := vars[0]
	assert.Nil(t, v.Validate("prod"), "prod should pass")
	assert.NotNil(t, v.Validate("production"), "production should fail, not in choices list")
}

func TestURLHTTPConditions_RoundTrip(t *testing.T) {
	// FormatValidiumData serializes conditions; ParseValidiumData reads them back;
	// Validate must honour them.
	source := []ValidiumVariable{
		{Name: "API", Type: "url-http", Conditions: json.RawMessage(`{"https":true}`)},
	}

	data := FormatValidiumData(source)

	vars, err := ParseValidiumData(data)
	require.NoError(t, err)
	require.Len(t, vars, 1)

	v := vars[0]
	assert.Nil(t, v.Validate("https://example.com"), "https:// should pass with HTTPSRequired=true")
	assert.NotNil(t, v.Validate("http://example.com"), "http:// should fail with HTTPSRequired=true")
}
