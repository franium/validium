package main

import (
	"fmt"
	"os"
)

var version = "0.2.0"

const (
	mainFilename        string = "validium.json"
	envFilename         string = ".env"
	envExampleFilename  string = ".env.example"
	envAgeFilename      string = ".env.age"
	ageIdentityFilename string = ".validium.key"
)

// command describes a CLI subcommand. needsEnv marks commands that cannot do
// anything useful until a .env file exists in the working directory.
type command struct {
	name        string
	description string
	needsEnv    bool
	run         func(args []string) error
}

// commandList is the single source of truth for supported subcommands.
// Its order is the order shown by help.
//
// Assigned in init instead of a var initializer: the help entry references
// help(), whose body reads commandList back, and Go rejects that as an
// initialization cycle when expressed as a package-level initializer.
var commandList []command

var commands map[string]command

func init() {
	commandList = []command{
		{
			name:        "init",
			description: "Initialize validium.json from your existing .env file.",
			needsEnv:    true,
			run: func([]string) error {
				variables, err := loadVariables()
				if err != nil {
					return fmt.Errorf("loading variables: %w", err)
				}
				return initialize(variables)
			},
		},
		{
			name:        "check",
			description: "Validate .env against validium.json or .env.example.",
			needsEnv:    true,
			run: func([]string) error {
				return check()
			},
		},
		{
			// generate reads validium.json and writes .env.example — does not need .env
			name:        "generate",
			description: "Generate .env.example from validium.json.",
			run: func([]string) error {
				return generate()
			},
		},
		{
			name:        "add",
			description: "Add a new variable to validium.json interactively.",
			run: func(args []string) error {
				if len(args) == 0 {
					return fmt.Errorf("no variable name provided; usage: validium add VARIABLE_NAME")
				}
				return add(args[0])
			},
		},
		{
			name:        "encrypt",
			description: "Encrypt .env to .env.age for one or more age recipients.",
			needsEnv:    true,
			run: func(args []string) error {
				if len(args) == 0 {
					return fmt.Errorf("no recipient provided; usage: validium encrypt AGE_RECIPIENT")
				}
				return encrypt(args)
			},
		},
		{
			name:        "decrypt",
			description: "Decrypt .env.age to .env using an age identity file.",
			run: func(args []string) error {
				identityFile := ageIdentityFilename
				if len(args) > 0 {
					identityFile = args[0]
				}
				return decrypt(identityFile)
			},
		},
		{
			name:        "keygen",
			description: "Generate a local age identity and print its public key.",
			run: func([]string) error {
				return keygen()
			},
		},
		{
			name:        "help",
			description: "Display this help message.",
			run: func([]string) error {
				help()
				return nil
			},
		},
	}

	commands = make(map[string]command, len(commandList))
	for _, c := range commandList {
		commands[c.name] = c
	}
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "No command provided. Use 'help' for usage information.")
		os.Exit(1)
	}

	name := args[0]
	if name == "--version" || name == "-v" {
		fmt.Printf("validium %s\n", version)
		return
	}

	cmd, ok := commands[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "Invalid command: %s. Use 'help' for usage information.\n", name)
		os.Exit(1)
	}

	if cmd.needsEnv && !fileExists(envFilename) {
		fmt.Fprintf(os.Stderr, "%s does not exist. Please create this file. There is nothing to check until this file exists.\n", envFilename)
		os.Exit(1)
	}

	if err := cmd.run(args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
