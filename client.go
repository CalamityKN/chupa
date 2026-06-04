package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func runClient(host string, port int) error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return err
	}
	defer conn.Close()

	fmt.Println("Connected to server.")
	return handleServerMessages(conn)
}

func handleServerMessages(conn net.Conn) error {
	for {
		message, err := ReceiveMessage(conn)
		if err != nil {
			return err
		}

		command := string(message)
		tokens, err := ParseCommandLine(command)
		if err != nil {
			if sendErr := SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err))); sendErr != nil {
				return sendErr
			}
			continue
		}
		if len(tokens) == 0 {
			continue
		}

		if tokens[0] == "exit" {
			return nil
		}

		if tokens[0] == "get" {
			if err := handleGetRequest(conn, tokens); err != nil {
				return err
			}
			continue
		}

		if tokens[0] == "put" {
			if err := handlePutRequest(conn, tokens); err != nil {
				return err
			}
			continue
		}

		if tokens[0] == "getdir" {
			if err := handleGetDirRequest(conn, tokens); err != nil {
				return err
			}
			continue
		}

		if tokens[0] == "putdir" {
			if err := handlePutDirRequest(conn, tokens); err != nil {
				return err
			}
			continue
		}

		response := handleCommand(tokens)
		if err := SendMessage(conn, []byte(response)); err != nil {
			return err
		}
	}
}

func handleCommand(tokens []string) string {
	switch tokens[0] {
	case "ping":
		return "pong"
	case "pwd":
		if len(tokens) != 1 {
			return "ERROR: usage: pwd"
		}
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		return wd
	case "cd":
		argument, err := singlePathArgument(tokens, "cd <path>")
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		if argument == "" {
			return "ERROR: missing path"
		}
		argument = normalizePath(argument)
		if err := os.Chdir(argument); err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		return fmt.Sprintf("OK: %s", wd)
	case "ls":
		argument, err := optionalPathArgument(tokens, "ls [path]")
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		return listDirectory(argument)
	case "mkdir":
		argument, err := singlePathArgument(tokens, "mkdir <path>")
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		if argument == "" {
			return "ERROR: missing path"
		}
		argument = normalizePath(argument)
		if err := os.MkdirAll(argument, 0755); err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		return "OK"
	case "hash":
		argument, err := singlePathArgument(tokens, "hash <path>")
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		return hashFile(argument)
	case "stat":
		argument, err := singlePathArgument(tokens, "stat <path>")
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		return statFile(argument)
	default:
		return "ERROR: unknown command"
	}
}

func handleGetRequest(conn net.Conn, tokens []string) error {
	if len(tokens) != 2 {
		return SendMessage(conn, []byte("ERROR: usage: get <remote_path>"))
	}

	path := tokens[1]
	path = normalizePath(path)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err)))
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err)))
	}

	if info.IsDir() {
		return SendMessage(conn, []byte("ERROR: path is a directory"))
	}

	file, err := os.Open(absPath)
	if err != nil {
		return SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err)))
	}
	file.Close()

	hash, err := ComputeSHA256(absPath)
	if err != nil {
		return SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err)))
	}

	if err := SendMessage(conn, []byte(fmt.Sprintf("OK %d %s", info.Size(), hash))); err != nil {
		return err
	}

	return SendFile(conn, absPath)
}

func handlePutRequest(conn net.Conn, tokens []string) error {
	remotePath, size, expectedHash, err := parsePutRequest(tokens)
	if err != nil {
		return SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err)))
	}

	remotePath = normalizePath(remotePath)
	absPath, err := filepath.Abs(remotePath)
	if err != nil {
		return SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err)))
	}

	if err := prepareOutputFile(absPath); err != nil {
		return SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err)))
	}

	if err := SendMessage(conn, []byte("READY")); err != nil {
		return err
	}

	received, err := ReceiveFile(conn, absPath, size)
	if err != nil {
		return SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err)))
	}

	actualHash, err := ComputeSHA256(absPath)
	if err != nil {
		return SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err)))
	}

	result := "SHA256 mismatch"
	if strings.EqualFold(actualHash, expectedHash) {
		result = "SHA256 verified"
	}

	return SendMessage(conn, []byte(fmt.Sprintf("OK: received %d bytes\n%s", received, result)))
}

func parsePutRequest(tokens []string) (string, int64, string, error) {
	if len(tokens) != 4 {
		return "", 0, "", fmt.Errorf("usage: put <remote_path> <size> <sha256>")
	}

	size, err := strconv.ParseInt(tokens[2], 10, 64)
	if err != nil {
		return "", 0, "", fmt.Errorf("invalid file size %q", tokens[2])
	}
	if size < 0 {
		return "", 0, "", fmt.Errorf("file size cannot be negative")
	}

	return tokens[1], size, tokens[3], nil
}

func handleGetDirRequest(conn net.Conn, tokens []string) error {
	if len(tokens) != 2 {
		return SendMessage(conn, []byte("ERROR: usage: getdir <remote_dir>"))
	}

	root, err := resolveClientDirPath(tokens[1])
	if err != nil {
		return SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err)))
	}

	entries, fileCount, totalBytes, err := BuildDirectoryManifest(root)
	if err != nil {
		return SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err)))
	}

	if err := SendMessage(conn, []byte(fmt.Sprintf("OK %d %d", fileCount, totalBytes))); err != nil {
		return err
	}
	if err := SendMessage(conn, []byte(SerializeManifest(entries))); err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.Kind != manifestFile {
			continue
		}

		source, err := safeJoin(root, entry.Rel)
		if err != nil {
			return err
		}
		if err := SendFile(conn, source); err != nil {
			return err
		}
	}

	return nil
}

func handlePutDirRequest(conn net.Conn, tokens []string) error {
	if len(tokens) != 2 {
		return SendMessage(conn, []byte("ERROR: usage: putdir <remote_dir>"))
	}

	root := normalizePath(tokens[1])
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err)))
	}

	if err := os.MkdirAll(rootAbs, 0755); err != nil {
		return SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err)))
	}

	if err := SendMessage(conn, []byte("READY")); err != nil {
		return err
	}

	manifestData, err := ReceiveMessage(conn)
	if err != nil {
		return err
	}

	entries, fileCount, totalBytes, err := ParseManifest(string(manifestData))
	if err != nil {
		return SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err)))
	}

	for _, entry := range entries {
		if entry.Kind != manifestDir {
			continue
		}
		target, err := safeJoin(rootAbs, entry.Rel)
		if err != nil {
			return SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err)))
		}
		if err := os.MkdirAll(target, 0755); err != nil {
			return SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err)))
		}
	}

	var receivedFiles int
	var receivedBytes int64
	for _, entry := range entries {
		if entry.Kind != manifestFile {
			continue
		}

		target, err := safeJoin(rootAbs, entry.Rel)
		if err != nil {
			return SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err)))
		}

		received, err := ReceiveFile(conn, target, entry.Size)
		if err != nil {
			return SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err)))
		}
		receivedBytes += received
		receivedFiles++

		actualHash, err := ComputeSHA256(target)
		if err != nil {
			return SendMessage(conn, []byte(fmt.Sprintf("ERROR: %v", err)))
		}
		if !strings.EqualFold(actualHash, entry.SHA256) {
			return SendMessage(conn, []byte(fmt.Sprintf("ERROR: SHA256 mismatch: %s", entry.Rel)))
		}
	}

	if receivedFiles != fileCount || receivedBytes != totalBytes {
		return SendMessage(conn, []byte("ERROR: manifest summary mismatch"))
	}

	return SendMessage(conn, []byte(fmt.Sprintf("OK: received %d files, %d bytes", receivedFiles, receivedBytes)))
}

func prepareOutputFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	return file.Close()
}

func singlePathArgument(tokens []string, usage string) (string, error) {
	if len(tokens) != 2 {
		return "", fmt.Errorf("usage: %s", usage)
	}
	return tokens[1], nil
}

func optionalPathArgument(tokens []string, usage string) (string, error) {
	if len(tokens) > 2 {
		return "", fmt.Errorf("usage: %s", usage)
	}
	if len(tokens) == 1 {
		return "", nil
	}
	return tokens[1], nil
}

func listDirectory(path string) string {
	if path == "" {
		path = "."
	}
	path = normalizePath(path)

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Sprintf("ERROR: %v", err)
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return fmt.Sprintf("ERROR: %v", err)
	}

	if len(entries) == 0 {
		return "(empty)"
	}

	var output strings.Builder
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}

		if entry.IsDir() {
			fmt.Fprintf(&output, "%-12s %s\n", "<DIR>", entry.Name())
			continue
		}

		fmt.Fprintf(&output, "%-12d %s\n", info.Size(), entry.Name())
	}

	return strings.TrimRight(output.String(), "\n")
}

func hashFile(path string) string {
	absPath, err := resolveClientFilePath(path)
	if err != nil {
		return fmt.Sprintf("ERROR: %v", err)
	}

	hash, err := ComputeSHA256(absPath)
	if err != nil {
		return fmt.Sprintf("ERROR: %v", err)
	}

	return fmt.Sprintf("SHA256: %s", hash)
}

func statFile(path string) string {
	absPath, err := resolveClientFilePath(path)
	if err != nil {
		return fmt.Sprintf("ERROR: %v", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Sprintf("ERROR: %v", err)
	}
	if info.IsDir() {
		return "ERROR: path is a directory"
	}

	hash, err := ComputeSHA256(absPath)
	if err != nil {
		return fmt.Sprintf("ERROR: %v", err)
	}

	return fmt.Sprintf("Path: %s\nSize: %d\nModified: %s\nSHA256: %s",
		absPath,
		info.Size(),
		info.ModTime().Format("2006-01-02 15:04:05"),
		hash,
	)
}

func resolveClientFilePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("missing path")
	}

	path = normalizePath(path)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory")
	}

	return absPath, nil
}

func resolveClientDirPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("missing path")
	}

	path = normalizePath(path)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory")
	}

	return absPath, nil
}

func normalizePath(path string) string {
	if runtime.GOOS != "windows" {
		return path
	}

	if len(path) == 2 && path[1] == ':' && isAsciiLetter(path[0]) {
		return path + `\`
	}

	return path
}

func isAsciiLetter(value byte) bool {
	return (value >= 'A' && value <= 'Z') || (value >= 'a' && value <= 'z')
}
