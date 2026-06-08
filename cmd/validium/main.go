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

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println("No command provided. Use 'help' for usage information.")
		os.Exit(1)
	}

	cmd := args[0]

	switch cmd {
	case "--version", "-v":
		fmt.Printf("validium %s\n", version)
		return
	case "help":
		help()
		return
	case "add":
		if len(args) < 2 {
			fmt.Println("No variable name provided. Usage: validium add VARIABLE_NAME")
			os.Exit(1)
		}
		if err := add(args[1]); err != nil {
			fmt.Printf("Error adding variable: %v\n", err)
			os.Exit(1)
		}
		return
	case "generate":
		// generate reads validium.json and writes .env.example — does not need .env
		if err := generate(); err != nil {
			fmt.Printf("Error generating: %v\n", err)
			os.Exit(1)
		}
		return
	case "keygen":
		if err := keygen(); err != nil {
			fmt.Printf("Error generating key: %v\n", err)
			os.Exit(1)
		}
		return
	case "decrypt":
		identityFile := ageIdentityFilename
		if len(args) > 1 {
			identityFile = args[1]
		}
		if err := decrypt(identityFile); err != nil {
			fmt.Printf("Error decrypting: %v\n", err)
			os.Exit(1)
		}
		return
	case "encrypt":
		if len(args) < 2 {
			fmt.Println("No recipient provided. Usage: validium encrypt AGE_RECIPIENT")
			os.Exit(1)
		}
		// requires .env — handled below
	case "init", "check":
		// requires .env — handled below
	default:
		fmt.Printf("Invalid command: %s. Use 'help' for usage information.\n", cmd)
		os.Exit(1)
	}

	if !fileExists(envFilename) {
		fmt.Printf("%s does not exist. Please create this file. There is nothing to check until this file exists.\n", envFilename)
		os.Exit(1)
	}

	switch cmd {
	case "init":
		variables, err := loadVariables()
		if err != nil {
			fmt.Printf("Error loading variables: %v\n", err)
			os.Exit(1)
		}
		if err := initialize(variables); err != nil {
			fmt.Printf("Error initializing: %v\n", err)
			os.Exit(1)
		}
	case "check":
		if err := check(); err != nil {
			os.Exit(1) // check() already rendered the error details
		}
	case "encrypt":
		if err := encrypt(args[1:]); err != nil {
			fmt.Printf("Error encrypting: %v\n", err)
			os.Exit(1)
		}

	}
}
