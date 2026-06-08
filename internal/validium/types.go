package validium

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

const SchemaVersion = "0.1.0"

type ValidiumValidationError struct {
	VariableName string
	Message      string
	Value        string
	Secret       bool
}

func (e ValidiumValidationError) Error() string {
	if e.VariableName != "" {
		return fmt.Sprintf("%s: %s", e.VariableName, e.Message)
	}
	return e.Message
}

type URLHTTPVariableConditions struct {
	HTTPSRequired bool `json:"https"`
}

type NumericVariableConditions struct {
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}
type StringVariableConditions struct {
	Choices []string `json:"choices,omitempty"`
}

// typeSpec is the single source of truth for a supported variable type:
// how to validate a raw value of that type, and how to infer it during `init`.
type typeSpec struct {
	name string
	// validate checks a non-empty raw value. conds carries the variable's raw
	// conditions JSON for types that support additional constraints.
	validate func(value string, conds json.RawMessage) error
	// infer reports whether a raw .env value should be classified as this type.
	// Evaluated in slice order, so order encodes inference precedence.
	infer func(value string) bool
}

// typeSpecs is ordered by inference precedence. "string" is the fallback and
// MUST stay last — its infer matches everything.
var typeSpecs = []typeSpec{
	{"boolean", validateBoolean, func(v string) bool { return v == "true" || v == "false" }},
	{"integer", validateInteger, isInteger},
	{"float", validateFloat, isFloat},
	{"url-http", validateURLHTTP, func(v string) bool {
		return validateURLHTTP(v, nil) == nil
	}},
	{"url", validateURL, func(v string) bool {
		return strings.Contains(v, "://") && validateURL(v, nil) == nil
	}},
	{"email", validateEmail, isLikelyEmail},
	{"string", validateString, func(v string) bool { return true }},
}

var (
	typeRegistry   = make(map[string]func(string, json.RawMessage) error, len(typeSpecs))
	AvailableTypes = make([]string, 0, len(typeSpecs))
)

func init() {
	for _, s := range typeSpecs {
		typeRegistry[s.name] = s.validate
		AvailableTypes = append(AvailableTypes, s.name)
	}
}

// IsKnownType reports whether t is a supported variable type.
func IsKnownType(t string) bool {
	_, ok := typeRegistry[t]
	return ok
}

// inferType picks the most specific type whose heuristic matches value.
func inferType(value string) string {
	for _, s := range typeSpecs {
		if s.infer(value) {
			return s.name
		}
	}
	return "string"
}

type ValidiumVariable struct {
	Name        string          `json:"-"`
	Type        string          `json:"type"`
	Description string          `json:"description,omitempty"`
	Default     any             `json:"default,omitempty"` // placeholder shown in generated .env.example; not applied as a fallback at validation time
	Secret      bool            `json:"secret"`
	Required    bool            `json:"required"`
	Conditions  json.RawMessage `json:"conditions,omitempty"`
}

func (v ValidiumVariable) Validate(actualValue string) *ValidiumValidationError {
	wrap := func(msg string) *ValidiumValidationError {
		return &ValidiumValidationError{
			VariableName: v.Name,
			Message:      msg,
			Value:        actualValue,
			Secret:       v.Secret,
		}
	}

	if v.Required && actualValue == "" {
		return wrap("required but missing")
	}
	if actualValue == "" {
		return nil // optional with empty value: present but intentionally blank
	}

	validate, ok := typeRegistry[v.Type]
	if !ok {
		return wrap(fmt.Sprintf("unknown type %q", v.Type))
	}
	if err := validate(actualValue, v.Conditions); err != nil {
		return wrap(err.Error())
	}
	return nil
}

type ValidiumData struct {
	Version   string                      `json:"version"`
	Variables map[string]ValidiumVariable `json:"variables"`
}

func FormatValidiumData(variables []ValidiumVariable) ValidiumData {
	result := ValidiumData{
		Version:   SchemaVersion,
		Variables: make(map[string]ValidiumVariable, len(variables)),
	}
	for _, v := range variables {
		result.Variables[v.Name] = v
	}
	return result
}

func ParseValidiumData(data ValidiumData) ([]ValidiumVariable, error) {
	variables := make([]ValidiumVariable, 0, len(data.Variables))
	for name, variable := range data.Variables {
		if !IsKnownType(variable.Type) {
			return nil, fmt.Errorf("unknown type %s for variable %s", variable.Type, name)
		}
		variable.Name = name
		variables = append(variables, variable)
	}
	slices.SortFunc(variables, func(a, b ValidiumVariable) int {
		return strings.Compare(a.Name, b.Name)
	})
	return variables, nil
}

func validateString(value string, conds json.RawMessage) error {
	if len(conds) == 0 {
		return nil
	}
	var c StringVariableConditions
	if err := json.Unmarshal(conds, &c); err != nil {
		return fmt.Errorf("invalid conditions for string: %w", err)
	}
	if len(c.Choices) > 0 && !slices.Contains(c.Choices, value) {
		return fmt.Errorf("value %q is not one of the allowed choices: %v", value, c.Choices)
	}

	return nil
}

func validateInteger(value string, conds json.RawMessage) error {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("invalid integer: %q", value)
	}
	if len(conds) == 0 {
		return nil
	}
	var c NumericVariableConditions
	if err := json.Unmarshal(conds, &c); err != nil {
		return fmt.Errorf("invalid conditions for integer: %w", err)
	}
	if c.Min != nil && float64(n) < *c.Min {
		return fmt.Errorf("value %d is below minimum %g", n, *c.Min)
	}
	if c.Max != nil && float64(n) > *c.Max {
		return fmt.Errorf("value %d is above maximum %g", n, *c.Max)
	}
	return nil
}

func validateFloat(value string, conds json.RawMessage) error {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("invalid float: %q", value)
	}
	if len(conds) == 0 {
		return nil
	}
	var c NumericVariableConditions
	if err := json.Unmarshal(conds, &c); err != nil {
		return fmt.Errorf("invalid conditions for float: %w", err)
	}
	if c.Min != nil && f < *c.Min {
		return fmt.Errorf("value %g is below minimum %g", f, *c.Min)
	}
	if c.Max != nil && f > *c.Max {
		return fmt.Errorf("value %g is above maximum %g", f, *c.Max)
	}
	return nil
}

func validateBoolean(value string, _ json.RawMessage) error {
	if value != "true" && value != "false" {
		return fmt.Errorf("invalid boolean")
	}
	return nil
}

func validateEmail(value string, _ json.RawMessage) error {
	addr, err := mail.ParseAddress(value)
	if err != nil {
		return fmt.Errorf("invalid email")
	}
	// Reject RFC 5322 display-name format: "Name <user@example.com>"
	if addr.Address != value {
		return fmt.Errorf("invalid email")
	}
	return nil
}

func validateURL(value string, _ json.RawMessage) error {
	if !strings.HasPrefix(value, "/") && !strings.Contains(value, "://") {
		return fmt.Errorf("invalid url")
	}
	if _, err := url.ParseRequestURI(value); err != nil {
		return fmt.Errorf("invalid url")
	}
	return nil
}

func validateURLHTTP(value string, conds json.RawMessage) error {
	var c URLHTTPVariableConditions
	if len(conds) > 0 {
		if err := json.Unmarshal(conds, &c); err != nil {
			return fmt.Errorf("invalid conditions for url-http: %w", err)
		}
	}
	if c.HTTPSRequired && !strings.HasPrefix(value, "https://") {
		return fmt.Errorf("url must start with https://")
	}
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		return fmt.Errorf("url must start with http:// or https://")
	}
	if _, err := url.ParseRequestURI(value); err != nil {
		return fmt.Errorf("invalid url")
	}
	return nil
}
