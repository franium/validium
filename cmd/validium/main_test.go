package main

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"filippo.io/age"
	"github.com/franium/validium/internal/validium"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cdTemp(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(orig) })
}

func writeFile(t *testing.T, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(name, []byte(content), 0644))
}

func TestInit_CreatesValidiumJSON(t *testing.T) {
	cdTemp(t)
	writeFile(t, ".env", "PORT=8080\nDEBUG=true\n")

	if err := os.Remove(mainFilename); err != nil && !os.IsNotExist(err) {
		require.NoError(t, err)
	}

	vars, err := loadVariables()
	require.NoError(t, err)
	require.NoError(t, initialize(vars))

	assert.FileExists(t, mainFilename, "validium.json was not created")
}

func TestInit_FailsWhenSchemaAlreadyExists(t *testing.T) {
	cdTemp(t)
	writeFile(t, ".env", "PORT=8080\n")
	writeFile(t, mainFilename, `{"version":"0.1.0","variables":{}}`)

	vars, _ := loadVariables()
	assert.Error(t, initialize(vars), "initialize() should fail when validium.json already exists")
}

func TestCheck_ValidEnvPassesAgainstSchema(t *testing.T) {
	cdTemp(t)
	writeFile(t, ".env", "PORT=8080\nDEBUG=true\n")
	writeFile(t, mainFilename, `{
		"version": "0.1.0",
		"variables": {
			"PORT":  {"type": "integer", "required": true},
			"DEBUG": {"type": "boolean", "required": true}
		}
	}`)

	assert.NoError(t, check(), "check() should pass for valid .env")
}

func TestCheck_MissingRequiredVarFails(t *testing.T) {
	cdTemp(t)
	writeFile(t, ".env", "DEBUG=true\n")
	writeFile(t, mainFilename, `{
		"version": "0.1.0",
		"variables": {
			"PORT":  {"type": "integer", "required": true},
			"DEBUG": {"type": "boolean", "required": true}
		}
	}`)

	assert.Error(t, check(), "check() should fail when required variable is missing")
}

func TestCheck_WrongTypeFails(t *testing.T) {
	cdTemp(t)
	writeFile(t, ".env", "PORT=notanumber\n")
	writeFile(t, mainFilename, `{
		"version": "0.1.0",
		"variables": {
			"PORT": {"type": "integer", "required": true}
		}
	}`)

	assert.Error(t, check(), "check() should fail when value does not match declared type")
}

func TestCheck_FallbackToEnvExample(t *testing.T) {
	cdTemp(t)
	writeFile(t, ".env", "FOO=bar\nBAZ=qux\n")
	writeFile(t, ".env.example", "FOO=\nBAZ=\n")

	assert.NoError(t, check(), "check() fallback should pass when all example keys are present")
}

func TestCheck_FallbackMissingKeyFails(t *testing.T) {
	cdTemp(t)
	writeFile(t, ".env", "FOO=bar\n")
	writeFile(t, ".env.example", "FOO=\nMISSING=\n")

	assert.Error(t, check(), "check() fallback should fail when a key from .env.example is absent in .env")
}

func TestCheck_OptionalVarAbsentFromEnvPasses(t *testing.T) {
	cdTemp(t)
	writeFile(t, ".env", "PORT=8080\n")
	writeFile(t, mainFilename, `{
		"version": "0.1.0",
		"variables": {
			"PORT":     {"type": "integer", "required": true},
			"OPTIONAL": {"type": "string",  "required": false}
		}
	}`)

	assert.NoError(t, check(), "check() should pass when optional variable is absent from .env")
}

func TestCheck_NoSpecFileFails(t *testing.T) {
	cdTemp(t)
	writeFile(t, ".env", "FOO=bar\n")

	assert.Error(t, check(), "check() should fail when neither validium.json nor .env.example exists")
}

func TestEncrypt_CreatesDecryptableEnvAge(t *testing.T) {
	cdTemp(t)
	writeFile(t, ".env", "SECRET=value\nPORT=8080\n")

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	require.NoError(t, encrypt([]string{identity.Recipient().String()}))

	file, err := os.Open(envAgeFilename)
	require.NoError(t, err, "%s was not created", envAgeFilename)
	defer file.Close()

	reader, err := age.Decrypt(file, identity)
	require.NoError(t, err)
	plaintext, err := io.ReadAll(reader)
	require.NoError(t, err)

	assert.Equal(t, "SECRET=value\nPORT=8080\n", string(plaintext))
}

func TestEncrypt_InvalidRecipientFails(t *testing.T) {
	cdTemp(t)
	writeFile(t, ".env", "SECRET=value\n")

	assert.Error(t, encrypt([]string{"not-an-age-recipient"}), "encrypt() should fail with an invalid recipient")
}

func TestEncrypt_PreservesExistingEnvAgeWhenEncryptionFails(t *testing.T) {
	cdTemp(t)
	require.NoError(t, os.Mkdir(envFilename, 0755))
	writeFile(t, envAgeFilename, "previous ciphertext")

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	assert.Error(t, encrypt([]string{identity.Recipient().String()}), "encrypt() should fail when .env cannot be read as a file")

	data, err := os.ReadFile(envAgeFilename)
	require.NoError(t, err)
	assert.Equal(t, "previous ciphertext", string(data), "failed encryption should preserve the previous .env.age")
}

func TestKeygen_CreatesParseableIdentityFile(t *testing.T) {
	cdTemp(t)

	require.NoError(t, keygen())

	identities, err := loadAgeIdentities(ageIdentityFilename)
	require.NoError(t, err)
	require.Len(t, identities, 1)

	info, err := os.Stat(ageIdentityFilename)
	require.NoError(t, err, "%s was not created", ageIdentityFilename)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestKeygen_RefusesToOverwriteIdentityFile(t *testing.T) {
	cdTemp(t)
	writeFile(t, ageIdentityFilename, "existing\n")

	assert.Error(t, keygen(), "keygen() should fail when the identity file already exists")
}

func TestDecrypt_CreatesEnvFromEnvAge(t *testing.T) {
	cdTemp(t)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	writeFile(t, ageIdentityFilename, identity.String()+"\n")
	writeEncryptedEnvAge(t, "SECRET=value\nPORT=8080\n", identity.Recipient())

	require.NoError(t, decrypt(ageIdentityFilename))

	plaintext, err := os.ReadFile(envFilename)
	require.NoError(t, err, "%s was not created", envFilename)
	assert.Equal(t, "SECRET=value\nPORT=8080\n", string(plaintext))
}

func TestDecrypt_RefusesToOverwriteEnv(t *testing.T) {
	cdTemp(t)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	writeFile(t, ageIdentityFilename, identity.String()+"\n")
	writeEncryptedEnvAge(t, "SECRET=value\n", identity.Recipient())
	writeFile(t, envFilename, "EXISTING=value\n")

	assert.Error(t, decrypt(ageIdentityFilename), "decrypt() should fail when .env already exists")
}

func writeEncryptedEnvAge(t *testing.T, plaintext string, recipient age.Recipient) {
	t.Helper()

	file, err := os.OpenFile(envAgeFilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	require.NoError(t, err, "creating %s", envAgeFilename)

	writer, err := age.Encrypt(file, recipient)
	if err != nil {
		_ = file.Close()
		require.NoError(t, err, "Encrypt()")
	}
	if _, err := writer.Write([]byte(plaintext)); err != nil {
		_ = writer.Close()
		_ = file.Close()
		require.NoError(t, err, "writing encrypted content")
	}
	require.NoError(t, writer.Close(), "closing encrypted writer")
	require.NoError(t, file.Close(), "closing %s", envAgeFilename)
}

func TestCheck_HttpsConditionEnforced(t *testing.T) {
	cdTemp(t)
	writeFile(t, ".env", "API_URL=http://example.com\n")
	writeFile(t, mainFilename, `{
		"version": "0.1.0",
		"variables": {
			"API_URL": {"type": "url-http", "required": true, "conditions": {"https": true}}
		}
	}`)

	assert.Error(t, check(), "check() should fail when https is required but value uses http://")
}

func TestCheck_HttpsConditionPermissive(t *testing.T) {
	cdTemp(t)
	writeFile(t, ".env", "API_URL=http://example.com\n")
	writeFile(t, mainFilename, `{
		"version": "0.1.0",
		"variables": {
			"API_URL": {"type": "url-http", "required": true, "conditions": {"https": false}}
		}
	}`)

	assert.NoError(t, check(), "check() should pass when https is not required")
}

func TestCheck_BothFilesPresent_UsesValidium(t *testing.T) {
	cdTemp(t)
	// .env.example lists EXTRA but validium.json does not — validium.json must win
	writeFile(t, ".env", "PORT=8080\n")
	writeFile(t, ".env.example", "PORT=\nEXTRA=\n")
	writeFile(t, mainFilename, `{
		"version": "0.1.0",
		"variables": {
			"PORT": {"type": "integer", "required": true}
		}
	}`)

	assert.NoError(t, check(), "check() should use validium.json and ignore extra keys in .env.example")
}

func TestGenerate_FailsWhenSchemaIsMissing(t *testing.T) {
	cdTemp(t)
	writeFile(t, ".env", "PORT=8080\n")

	assert.Error(t, generate(), "generate() should fail when validium.json does not exist")
}

func TestGenerate_CreatesEnvExample(t *testing.T) {
	cdTemp(t)
	writeFile(t, mainFilename, `{
		"version": "0.1.0",
		"variables": {
			"PORT":  {"type": "integer", "description": "HTTP port"},
			"DEBUG": {"type": "boolean", "description": ""}
		}
	}`)

	require.NoError(t, generate())

	data, err := os.ReadFile(envExampleFilename)
	require.NoError(t, err, ".env.example was not created")

	content := string(data)
	assert.Contains(t, content, "PORT=")
	assert.Contains(t, content, "DEBUG=")
	assert.Contains(t, content, "HTTP port", ".env.example missing description comment for PORT")
}

func TestGenerate_EmitsDefaultValues(t *testing.T) {
	cdTemp(t)
	writeFile(t, mainFilename, `{
		"version": "0.1.0",
		"variables": {
			"PORT":  {"type": "integer", "default": 8080},
			"NAME":  {"type": "string",  "default": "hello"},
			"RATIO": {"type": "float",   "default": 3.14},
			"FLAG":  {"type": "boolean", "default": true}
		}
	}`)

	require.NoError(t, generate())

	data, err := os.ReadFile(envExampleFilename)
	require.NoError(t, err, ".env.example was not created")
	content := string(data)

	for _, want := range []string{"PORT=8080", "NAME=hello", "RATIO=3.14", "FLAG=true"} {
		assert.Contains(t, content, want)
	}
}

func TestGenerate_NoDefaultEmitsEmptyPlaceholder(t *testing.T) {
	cdTemp(t)
	writeFile(t, mainFilename, `{
		"version": "0.1.0",
		"variables": {
			"DEBUG": {"type": "boolean"}
		}
	}`)

	require.NoError(t, generate())

	data, err := os.ReadFile(envExampleFilename)
	require.NoError(t, err, ".env.example was not created")
	assert.Contains(t, string(data), "DEBUG=\n", "expected bare 'DEBUG=' placeholder")
}

func TestGenerate_OverwritesExisting(t *testing.T) {
	cdTemp(t)
	writeFile(t, envExampleFilename, "OLD_KEY=stale\n")
	writeFile(t, mainFilename, `{
		"version": "0.1.0",
		"variables": {
			"NEW_KEY": {"type": "string"}
		}
	}`)

	require.NoError(t, generate())

	data, err := os.ReadFile(envExampleFilename)
	require.NoError(t, err, ".env.example was not created")
	content := string(data)
	assert.NotContains(t, content, "OLD_KEY", "generate() should overwrite stale keys")
	assert.Contains(t, content, "NEW_KEY=")
}

func TestAdd_FailsWhenSchemaMissing(t *testing.T) {
	cdTemp(t)

	assert.Error(t, add("FOO"), "add() should fail when validium.json does not exist")
}

func TestAdd_FailsWhenNameEmpty(t *testing.T) {
	cdTemp(t)
	writeFile(t, mainFilename, `{"version":"0.1.0","variables":{}}`)

	assert.Error(t, add("   "), "add() should fail when variable name is blank")
}

func TestAdd_ExistingVariableMakesNoChange(t *testing.T) {
	cdTemp(t)
	original := `{"version":"0.1.0","variables":{"PORT":{"type":"integer","secret":false,"required":true}}}`
	writeFile(t, mainFilename, original)

	// PORT already exists, so add returns before reaching the interactive form.
	assert.NoError(t, add("PORT"), "add() of an existing variable should not error")

	data, err := loadValidiumData()
	require.NoError(t, err)
	assert.Len(t, data.Variables, 1, "expected schema unchanged with 1 variable")
}

func TestBuildNumericConditions(t *testing.T) {
	tests := []struct {
		name    string
		min     string
		max     string
		wantNil bool
		wantErr bool
	}{
		{name: "both empty returns nil", min: "", max: "", wantNil: true},
		{name: "min only", min: "1", max: "", wantNil: false},
		{name: "max only", min: "", max: "100", wantNil: false},
		{name: "both set", min: "1", max: "100", wantNil: false},
		{name: "whitespace treated as empty", min: "  ", max: "  ", wantNil: true},
		{name: "invalid min", min: "abc", max: "", wantErr: true},
		{name: "invalid max", min: "", max: "abc", wantErr: true},
		{name: "float values accepted", min: "0.5", max: "1.5", wantNil: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildNumericConditions(tt.min, tt.max)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
			}
		})
	}
}

func TestBuildStringConditions(t *testing.T) {
	tests := []struct {
		name        string
		hasChoices  bool
		choices     string
		wantNil     bool
		wantErr     bool
		wantChoices []string
	}{
		{name: "choices disabled returns nil", hasChoices: false, choices: "dev,prod", wantNil: true},
		{name: "comma separated choices", hasChoices: true, choices: "dev,staging,prod", wantChoices: []string{"dev", "staging", "prod"}},
		{name: "trims whitespace", hasChoices: true, choices: " dev, staging , prod ", wantChoices: []string{"dev", "staging", "prod"}},
		{name: "ignores empty items", hasChoices: true, choices: "dev,,prod,", wantChoices: []string{"dev", "prod"}},
		{name: "enabled empty choices fails", hasChoices: true, choices: " , ", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildStringConditions(tt.hasChoices, tt.choices)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, got)
				return
			}

			var conds validium.StringVariableConditions
			require.NoError(t, json.Unmarshal(got, &conds), "conditions should be valid JSON")
			assert.Equal(t, tt.wantChoices, conds.Choices)
		})
	}
}

func TestCheck_IntegerMinConditionEnforced(t *testing.T) {
	cdTemp(t)
	writeFile(t, ".env", "PORT=0\n")
	writeFile(t, mainFilename, `{
		"version": "0.1.0",
		"variables": {
			"PORT": {"type": "integer", "required": true, "conditions": {"min": 1}}
		}
	}`)

	assert.Error(t, check(), "check() should fail when integer is below minimum")
}

func TestCheck_IntegerMaxConditionEnforced(t *testing.T) {
	cdTemp(t)
	writeFile(t, ".env", "PORT=99999\n")
	writeFile(t, mainFilename, `{
		"version": "0.1.0",
		"variables": {
			"PORT": {"type": "integer", "required": true, "conditions": {"max": 65535}}
		}
	}`)

	assert.Error(t, check(), "check() should fail when integer is above maximum")
}

func TestCheck_IntegerWithinRangePasses(t *testing.T) {
	cdTemp(t)
	writeFile(t, ".env", "PORT=8080\n")
	writeFile(t, mainFilename, `{
		"version": "0.1.0",
		"variables": {
			"PORT": {"type": "integer", "required": true, "conditions": {"min": 1, "max": 65535}}
		}
	}`)

	assert.NoError(t, check(), "check() should pass when integer is within range")
}

func TestCheck_FloatRangeEnforced(t *testing.T) {
	cdTemp(t)
	writeFile(t, ".env", "RATIO=1.5\n")
	writeFile(t, mainFilename, `{
		"version": "0.1.0",
		"variables": {
			"RATIO": {"type": "float", "required": true, "conditions": {"min": 0.0, "max": 1.0}}
		}
	}`)

	assert.Error(t, check(), "check() should fail when float is above maximum")
}

func TestWriteVariable_PersistsNewVariable(t *testing.T) {
	cdTemp(t)
	writeFile(t, mainFilename, `{"version":"0.1.0","variables":{"PORT":{"type":"integer","required":true}}}`)

	data, err := loadValidiumData()
	require.NoError(t, err)

	newVar := validium.ValidiumVariable{Name: "API_KEY", Type: "string", Required: true, Secret: true}
	require.NoError(t, writeVariable(data, "API_KEY", newVar))

	reloaded, err := loadValidiumData()
	require.NoError(t, err)

	got, ok := reloaded.Variables["API_KEY"]
	require.True(t, ok, "API_KEY was not persisted")
	assert.Equal(t, "string", got.Type)
	assert.True(t, got.Required)
	assert.True(t, got.Secret)
	assert.Contains(t, reloaded.Variables, "PORT", "writeVariable() dropped the pre-existing PORT variable")
}
