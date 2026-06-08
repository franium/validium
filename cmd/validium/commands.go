package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"charm.land/huh/v2"
	"filippo.io/age"
	"github.com/franium/validium/internal/validium"
)

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func loadValidiumData() (validium.ValidiumData, error) {
	var data validium.ValidiumData
	file, err := os.Open(mainFilename)
	if err != nil {
		return data, fmt.Errorf("error opening %s: %w", mainFilename, err)
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return data, fmt.Errorf("error decoding JSON from %s: %w", mainFilename, err)
	}
	return data, nil
}

func loadVariables() ([]validium.ValidiumVariable, error) {
	return validium.IdentifyVariables(envFilename)
}

func initialize(variables []validium.ValidiumVariable) error {
	// Defensive check: main.go also guards this, but Init may be called directly in tests or future callers.
	if !fileExists(envFilename) {
		return fmt.Errorf("%s does not exist. Please create this file before initializing.", envFilename)
	}
	if fileExists(mainFilename) {
		return fmt.Errorf("%s already exists. Initialization has already been done. Use 'generate'.", mainFilename)
	}
	return writeSchema(variables)
}

func checkEnvExample() error {
	if !fileExists(envExampleFilename) {
		return fmt.Errorf("%s does not exist. Please create this file before checking.", envExampleFilename)
	}

	exampleVars, err := validium.ParseEnvFile(envExampleFilename)
	if err != nil {
		return fmt.Errorf("error reading %s: %w", envExampleFilename, err)
	}

	envVars, err := validium.ParseEnvFile(envFilename)
	if err != nil {
		return fmt.Errorf("error reading %s: %w", envFilename, err)
	}

	exampleKeys := make([]string, 0, len(exampleVars))
	for name := range exampleVars {
		exampleKeys = append(exampleKeys, name)
	}
	slices.Sort(exampleKeys)

	var errs []validium.ValidiumValidationError
	for _, name := range exampleKeys {
		if _, found := envVars[name]; !found {
			errs = append(errs, validium.ValidiumValidationError{
				VariableName: name,
				Message:      "variable defined in .env.example is missing in .env file",
			})
		}
	}

	if len(errs) > 0 {
		renderValidationErrors(errs)
		return fmt.Errorf("validation failed with %d missing variable(s)", len(errs))
	}

	renderSuccess(`All variables defined in .env.example are present in .env.
Remember that .env.example is just a template and does not enforce any validation rules.
In case you want to enforce validation rules, please create a validium.json file
using the 'init' command.`)
	return nil
}

func checkValidium() error {
	data, err := loadValidiumData()
	if err != nil {
		return err
	}

	validiumVars, err := validium.ParseValidiumData(data)
	if err != nil {
		return fmt.Errorf("error parsing Validium data: %w", err)
	}

	envVars, err := validium.ParseEnvFile(envFilename)
	if err != nil {
		return fmt.Errorf("error parsing %s: %w", envFilename, err)
	}

	var errs []validium.ValidiumValidationError
	for _, validiumVar := range validiumVars {
		val, found := envVars[validiumVar.Name]
		if !found {
			if validiumVar.Required {
				errs = append(errs, validium.ValidiumValidationError{
					VariableName: validiumVar.Name,
					Message:      "variable not found in .env file",
				})
			}
			continue
		}
		if err := validiumVar.Validate(val); err != nil {
			errs = append(errs, *err)
		}
	}

	if len(errs) > 0 {
		renderValidationErrors(errs)
		return fmt.Errorf("validation failed with %d error(s)", len(errs))
	}

	renderSuccess("All variables are valid according to the definitions in validium.json.")
	return nil
}

func check() error {
	mainExists := fileExists(mainFilename)
	envExampleExists := fileExists(envExampleFilename)

	switch {
	case mainExists:
		// checkValidium already renders validation details; main.go only inspects
		// whether the returned error is non-nil. No extra wrapping needed.
		return checkValidium()
	case envExampleExists:
		return checkEnvExample()
	default:
		return fmt.Errorf("neither %s nor %s exists. Please create one of these files", mainFilename, envExampleFilename)
	}
}

func encrypt(recipientKeys []string) error {
	if len(recipientKeys) == 0 {
		return fmt.Errorf("at least one age recipient is required")
	}

	input, err := os.Open(envFilename)
	if err != nil {
		return fmt.Errorf("error opening %s: %w", envFilename, err)
	}
	defer input.Close()

	recipients := make([]age.Recipient, 0, len(recipientKeys))
	for _, key := range recipientKeys {
		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("age recipient cannot be empty")
		}
		recipient, err := age.ParseX25519Recipient(key)
		if err != nil {
			return fmt.Errorf("invalid age recipient %q: %w", key, err)
		}
		recipients = append(recipients, recipient)
	}

	output, err := os.CreateTemp(".", envAgeFilename+".tmp-*")
	if err != nil {
		return fmt.Errorf("error creating temporary encrypted file: %w", err)
	}
	tmpName := output.Name()
	defer os.Remove(tmpName)

	encryptedWriter, err := age.Encrypt(output, recipients...)
	if err != nil {
		_ = output.Close()
		return fmt.Errorf("error initializing encryption: %w", err)
	}

	if _, err := io.Copy(encryptedWriter, input); err != nil {
		_ = encryptedWriter.Close()
		_ = output.Close()
		return fmt.Errorf("error encrypting %s: %w", envFilename, err)
	}
	if err := encryptedWriter.Close(); err != nil {
		_ = output.Close()
		return fmt.Errorf("error finalizing encryption: %w", err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("error closing temporary encrypted file: %w", err)
	}
	if err := os.Rename(tmpName, envAgeFilename); err != nil {
		return fmt.Errorf("error replacing %s: %w", envAgeFilename, err)
	}

	fmt.Printf("%s has been encrypted to %s.\n", envFilename, envAgeFilename)
	return nil
}

func keygen() error {
	if fileExists(ageIdentityFilename) {
		return fmt.Errorf("%s already exists. Refusing to overwrite an existing private key", ageIdentityFilename)
	}

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return fmt.Errorf("error generating age identity: %w", err)
	}

	file, err := os.CreateTemp(".", ageIdentityFilename+".tmp-*")
	if err != nil {
		return fmt.Errorf("error creating temporary identity file: %w", err)
	}
	tmpName := file.Name()
	defer os.Remove(tmpName)

	content := fmt.Sprintf("# public key: %s\n%s\n", identity.Recipient(), identity)
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		return fmt.Errorf("error writing temporary identity file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("error closing temporary identity file: %w", err)
	}
	if err := os.Link(tmpName, ageIdentityFilename); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%s already exists. Refusing to overwrite an existing private key", ageIdentityFilename)
		}
		return fmt.Errorf("error creating %s: %w", ageIdentityFilename, err)
	}

	fmt.Printf("Identity written to %s.\nPublic key: %s\n", ageIdentityFilename, identity.Recipient())
	return nil
}

func decrypt(identityFilename string) error {
	identityFilename = strings.TrimSpace(identityFilename)
	if identityFilename == "" {
		return fmt.Errorf("age identity file cannot be empty")
	}
	if fileExists(envFilename) {
		return fmt.Errorf("%s already exists. Refusing to overwrite it", envFilename)
	}

	identities, err := loadAgeIdentities(identityFilename)
	if err != nil {
		return err
	}

	input, err := os.Open(envAgeFilename)
	if err != nil {
		return fmt.Errorf("error opening %s: %w", envAgeFilename, err)
	}
	defer input.Close()

	decryptedReader, err := age.Decrypt(input, identities...)
	if err != nil {
		return fmt.Errorf("error initializing decryption: %w", err)
	}

	output, err := os.OpenFile(envFilename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("error creating %s: %w", envFilename, err)
	}

	if _, err := io.Copy(output, decryptedReader); err != nil {
		_ = output.Close()
		_ = os.Remove(envFilename)
		return fmt.Errorf("error decrypting %s: %w", envAgeFilename, err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("error closing %s: %w", envFilename, err)
	}

	fmt.Printf("%s has been decrypted to %s.\n", envAgeFilename, envFilename)
	return nil
}

func loadAgeIdentities(identityFilename string) ([]age.Identity, error) {
	content, err := os.ReadFile(identityFilename)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", identityFilename, err)
	}

	var identities []age.Identity
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		identity, err := age.ParseX25519Identity(line)
		if err != nil {
			return nil, fmt.Errorf("invalid age identity in %s: %w", identityFilename, err)
		}
		identities = append(identities, identity)
	}

	if len(identities) == 0 {
		return nil, fmt.Errorf("no age identities found in %s", identityFilename)
	}
	return identities, nil
}

func writeSchema(variables []validium.ValidiumVariable) error {
	if len(variables) == 0 {
		return fmt.Errorf("no variables found in %s", envFilename)
	}

	data := validium.FormatValidiumData(variables)
	dataJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	if err := os.WriteFile(mainFilename, dataJSON, 0644); err != nil {
		return fmt.Errorf("error writing %s: %w", mainFilename, err)
	}

	fmt.Printf("%s has been generated successfully.\n", mainFilename)
	return nil
}

func generate() error {
	if !fileExists(mainFilename) {
		return fmt.Errorf("%s not found. Run 'init' first to generate it from your .env", mainFilename)
	}

	data, err := loadValidiumData()
	if err != nil {
		return err
	}

	keys := make([]string, 0, len(data.Variables))
	for name := range data.Variables {
		keys = append(keys, name)
	}
	slices.Sort(keys)

	var sb strings.Builder
	sb.WriteString("# Generated by validium — do not add real values here\n")
	for _, name := range keys {
		v := data.Variables[name]
		if v.Description != "" {
			sb.WriteString("# " + v.Description + "\n")
		}
		sb.WriteString(name + "=" + formatDefaultValue(v.Default) + "\n")
	}

	existed := fileExists(envExampleFilename)
	if err := os.WriteFile(envExampleFilename, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("error writing %s: %w", envExampleFilename, err)
	}

	if existed {
		fmt.Printf("%s has been updated.\n", envExampleFilename)
	} else {
		fmt.Printf("%s has been generated successfully.\n", envExampleFilename)
	}
	return nil
}

// formatDefaultValue renders a schema default as the placeholder value for
// .env.example. A nil default (no "default" key in the schema) yields an empty
// placeholder. JSON numbers arrive as float64; %v prints integers without a
// trailing ".0" and decimals as-is.
func formatDefaultValue(d any) string {
	if d == nil {
		return ""
	}
	return fmt.Sprintf("%v", d)
}

func help() {
	fmt.Println("Usage: validium [command]")
	fmt.Println("Commands:")
	fmt.Println("  init      - Initialize validium.json from your existing .env file.")
	fmt.Println("  check     - Validate .env against validium.json or .env.example.")
	fmt.Println("  generate  - Generate .env.example from validium.json.")
	fmt.Println("  add       - Add a new variable to validium.json interactively.")
	fmt.Println("  encrypt   - Encrypt .env to .env.age for one or more age recipients.")
	fmt.Println("  decrypt   - Decrypt .env.age to .env using an age identity file.")
	fmt.Println("  keygen    - Generate a local age identity and print its public key.")
	fmt.Println("  help      - Display this help message.")
}

func add(variableName string) error {
	variableName = strings.ToUpper(variableName)

	if !fileExists(mainFilename) {
		return fmt.Errorf("%s not found. Run 'init' first to generate it from your .env", mainFilename)
	}
	if strings.TrimSpace(variableName) == "" {
		return fmt.Errorf("variable name cannot be empty")
	}
	data, err := loadValidiumData()
	if err != nil {
		return err
	}

	if _, exists := data.Variables[variableName]; exists {
		renderWarning(fmt.Sprintf("Variable %q already exists in %s. No changes made.", variableName, mainFilename))
		return nil
	}

	variable, err := addVariableForm(variableName)
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Println("Cancelled.")
			return nil
		}
		return fmt.Errorf("error collecting variable options: %w", err)
	}

	if err := writeVariable(data, variableName, variable); err != nil {
		return err
	}

	msg := fmt.Sprintf("Variable %s added successfully. Please run 'generate' to update your .env.example file.", variableName)
	renderSuccess(msg)

	return nil
}

// writeVariable inserts variable into data under variableName and persists the
// schema. Kept separate from add's interactive form so the persistence path is
// testable without a TTY.
func writeVariable(data validium.ValidiumData, variableName string, variable validium.ValidiumVariable) error {
	data.Variables[variableName] = variable

	dataJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}
	if err := os.WriteFile(mainFilename, dataJSON, 0644); err != nil {
		return fmt.Errorf("error writing to %s: %w", mainFilename, err)
	}
	return nil
}

func addVariableForm(variableName string) (validium.ValidiumVariable, error) {
	var selectedType string
	var required bool
	var secret bool

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select the type of your new variable").
				Options(
					huh.NewOption("String", "string"),
					huh.NewOption("Number", "integer"),
					huh.NewOption("Float", "float"),
					huh.NewOption("Boolean", "boolean"),
					huh.NewOption("Url", "url"),
					huh.NewOption("HTTP Url", "url-http"),
					huh.NewOption("Email", "email"),
				).Value(&selectedType),
			huh.NewConfirm().
				Title("Is this variable required?").
				Value(&required),
			huh.NewConfirm().
				Title("Is this variable secret?").
				Value(&secret),
		),
	)
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return validium.ValidiumVariable{}, huh.ErrUserAborted
		}
		return validium.ValidiumVariable{}, fmt.Errorf("error running form: %w", err)
	}

	variable := validium.ValidiumVariable{
		Name:     variableName,
		Type:     selectedType,
		Required: required,
		Secret:   secret,
	}

	if selectedType == "integer" || selectedType == "float" {
		var minStr, maxStr string
		numForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Minimum value (leave empty for no minimum)").
					Value(&minStr),
				huh.NewInput().
					Title("Maximum value (leave empty for no maximum)").
					Value(&maxStr),
			),
		)
		if err := numForm.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return validium.ValidiumVariable{}, huh.ErrUserAborted
			}
			return validium.ValidiumVariable{}, fmt.Errorf("error running form: %w", err)
		}
		conds, err := buildNumericConditions(minStr, maxStr)
		if err != nil {
			return validium.ValidiumVariable{}, err
		}
		variable.Conditions = conds
	}

	if selectedType == "string" {
		var hasChoices bool
		var choicesStr string
		choicesConfirmForm := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Does this string have allowed choices?").
					Value(&hasChoices),
			),
		)
		if err := choicesConfirmForm.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return validium.ValidiumVariable{}, huh.ErrUserAborted
			}
			return validium.ValidiumVariable{}, fmt.Errorf("error running form: %w", err)
		}
		if hasChoices {
			choicesInputForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Allowed choices, separated by commas").
						Value(&choicesStr).
						Validate(func(value string) error {
							return validateChoicesInput(value)
						}),
				),
			)
			if err := choicesInputForm.Run(); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					return validium.ValidiumVariable{}, huh.ErrUserAborted
				}
				return validium.ValidiumVariable{}, fmt.Errorf("error running form: %w", err)
			}
		}
		conds, err := buildStringConditions(hasChoices, choicesStr)
		if err != nil {
			return validium.ValidiumVariable{}, err
		}
		variable.Conditions = conds
	}

	return variable, nil
}

func buildStringConditions(hasChoices bool, choicesStr string) (json.RawMessage, error) {
	if !hasChoices {
		return nil, nil
	}
	if err := validateChoicesInput(choicesStr); err != nil {
		return nil, err
	}

	choices := parseCommaSeparatedChoices(choicesStr)
	c := validium.StringVariableConditions{Choices: choices}
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func validateChoicesInput(choicesStr string) error {
	if len(parseCommaSeparatedChoices(choicesStr)) == 0 {
		return fmt.Errorf("choices cannot be empty when enabled")
	}
	return nil
}

func parseCommaSeparatedChoices(choicesStr string) []string {
	parts := strings.Split(choicesStr, ",")
	choices := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			choices = append(choices, part)
		}
	}
	return choices
}

func buildNumericConditions(minStr, maxStr string) (json.RawMessage, error) {
	minStr = strings.TrimSpace(minStr)
	maxStr = strings.TrimSpace(maxStr)
	if minStr == "" && maxStr == "" {
		return nil, nil
	}
	c := validium.NumericVariableConditions{}
	if minStr != "" {
		v, err := strconv.ParseFloat(minStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid minimum value %q: must be a number", minStr)
		}
		c.Min = &v
	}
	if maxStr != "" {
		v, err := strconv.ParseFloat(maxStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid maximum value %q: must be a number", maxStr)
		}
		c.Max = &v
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	return b, nil
}
