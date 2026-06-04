package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const AppName = "chupa"
const Version = "0.1.0"

func main() {
	if len(os.Args) == 2 && isVersionArg(os.Args[1]) {
		fmt.Printf("%s %s\n", AppName, Version)
		return
	}

	if len(os.Args) == 3 && os.Args[1] == "server" {
		port, err := parsePort(os.Args[2])
		if err != nil {
			fatalUsage(err)
		}

		if err := runServer(port); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(os.Args) == 3 {
		host := os.Args[1]
		port, err := parsePort(os.Args[2])
		if err != nil {
			fatalUsage(err)
		}

		if err := runClient(host, port); err != nil {
			fmt.Fprintf(os.Stderr, "client error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fatalUsage(nil)
}

func isVersionArg(value string) bool {
	return value == "version" || value == "--version" || value == "-v"
}

func parsePort(value string) (int, error) {
	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid port %q", value)
	}

	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port must be between 1 and 65535")
	}

	return port, nil
}

func fatalUsage(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n\n", err)
	}

	fmt.Fprintf(os.Stderr, "usage:\n")
	fmt.Fprintf(os.Stderr, "  %s server <port>\n", AppName)
	fmt.Fprintf(os.Stderr, "  %s <ip> <port>\n", AppName)
	os.Exit(2)
}

func ParseCommandLine(input string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	inQuotes := false
	tokenStarted := false

	for _, ch := range input {
		switch {
		case ch == '"':
			inQuotes = !inQuotes
			tokenStarted = true
		case ch == ' ' || ch == '\t':
			if inQuotes {
				current.WriteRune(ch)
				continue
			}
			if tokenStarted {
				tokens = append(tokens, current.String())
				current.Reset()
				tokenStarted = false
			}
		default:
			current.WriteRune(ch)
			tokenStarted = true
		}
	}

	if inQuotes {
		return nil, fmt.Errorf("unmatched quote")
	}

	if tokenStarted {
		tokens = append(tokens, current.String())
	}

	return tokens, nil
}

func QuoteArgument(value string) string {
	if value == "" {
		return `""`
	}
	if !strings.ContainsAny(value, " \t\"") {
		return value
	}

	return `"` + value + `"`
}
