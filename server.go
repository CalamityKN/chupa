package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func runServer(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		fmt.Printf("Client connected: %s\n", conn.RemoteAddr())
		if err := interactiveSession(conn); err != nil {
			fmt.Fprintf(os.Stderr, "session ended: %v\n", err)
		}
		conn.Close()
	}
}

func interactiveSession(conn net.Conn) error {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("tool> ")
		if !scanner.Scan() {
			return scanner.Err()
		}

		command := strings.TrimSpace(scanner.Text())
		if command == "" {
			continue
		}

		tokens, err := ParseCommandLine(command)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			continue
		}
		if len(tokens) == 0 {
			continue
		}

		switch tokens[0] {
		case "help":
			printHelp()
			continue
		case "get":
			if err := handleGet(conn, tokens); err != nil {
				fmt.Printf("ERROR: %v\n", err)
			}
			continue
		case "put":
			if err := handlePut(conn, tokens); err != nil {
				fmt.Printf("ERROR: %v\n", err)
			}
			continue
		case "getdir":
			if err := handleGetDir(conn, tokens); err != nil {
				fmt.Printf("ERROR: %v\n", err)
			}
			continue
		case "putdir":
			if err := handlePutDir(conn, tokens); err != nil {
				fmt.Printf("ERROR: %v\n", err)
			}
			continue
		}

		if err := SendMessage(conn, []byte(command)); err != nil {
			return err
		}

		if command == "exit" {
			return nil
		}

		response, err := ReceiveMessage(conn)
		if err != nil {
			return err
		}
		fmt.Println(string(response))
	}
}

func handleGet(conn net.Conn, tokens []string) error {
	remotePath, localPath, err := parseGetCommand(tokens)
	if err != nil {
		return err
	}

	localPath, err = resolveLocalGetPath(remotePath, localPath)
	if err != nil {
		return err
	}

	fmt.Printf("Requesting: %s\n", remotePath)
	fmt.Printf("Saving to: %s\n", localPath)

	if err := SendMessage(conn, []byte("get "+QuoteArgument(remotePath))); err != nil {
		return err
	}

	response, err := ReceiveMessage(conn)
	if err != nil {
		return err
	}

	responseText := string(response)
	if strings.HasPrefix(responseText, "ERROR:") {
		fmt.Println(responseText)
		return nil
	}

	responseParts := strings.Fields(responseText)
	if len(responseParts) != 3 || responseParts[0] != "OK" {
		return fmt.Errorf("unexpected response: %s", responseText)
	}

	size, err := strconv.ParseInt(responseParts[1], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid file size %q", responseParts[1])
	}
	expectedHash := responseParts[2]

	progress := newProgressPrinter()
	received, err := ReceiveFileWithProgress(conn, localPath, size, progress)
	if err != nil {
		return err
	}
	fmt.Println()

	fmt.Printf("Received %d bytes\n", received)
	actualHash, err := ComputeSHA256(localPath)
	if err != nil {
		return err
	}
	if strings.EqualFold(actualHash, expectedHash) {
		fmt.Println("SHA256 verified")
	} else {
		fmt.Println("SHA256 mismatch")
	}
	return nil
}

func parseGetCommand(tokens []string) (string, string, error) {
	if len(tokens) < 2 {
		return "", "", fmt.Errorf("usage: get <remote_path> [local_path]")
	}
	if len(tokens) > 3 {
		return "", "", fmt.Errorf("usage: get <remote_path> [local_path]")
	}

	remotePath := tokens[1]
	localPath := ""
	if len(tokens) == 3 {
		localPath = tokens[2]
	}

	return remotePath, localPath, nil
}

func handlePut(conn net.Conn, tokens []string) error {
	localPath, remotePath, err := parsePutCommand(tokens)
	if err != nil {
		return err
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("local path is not a regular file")
	}

	hash, err := ComputeSHA256(localPath)
	if err != nil {
		return err
	}

	remotePath = resolveRemotePutPath(localPath, remotePath)

	fmt.Printf("Uploading: %s\n", localPath)
	fmt.Printf("Remote path: %s\n", remotePath)

	if err := SendMessage(conn, []byte(fmt.Sprintf("put %s %d %s", QuoteArgument(remotePath), info.Size(), hash))); err != nil {
		return err
	}

	response, err := ReceiveMessage(conn)
	if err != nil {
		return err
	}

	responseText := string(response)
	if responseText != "READY" {
		fmt.Println(responseText)
		return nil
	}

	progress := newProgressPrinter()
	if err := SendFileWithProgress(conn, localPath, progress); err != nil {
		return err
	}
	fmt.Println()

	finalResponse, err := ReceiveMessage(conn)
	if err != nil {
		return err
	}

	finalResponseText := string(finalResponse)
	if strings.HasPrefix(finalResponseText, "ERROR:") {
		fmt.Println(finalResponseText)
		return nil
	}

	fmt.Printf("Sent %d bytes\n", info.Size())
	if strings.Contains(finalResponseText, "SHA256 verified") {
		fmt.Println("SHA256 verified")
	} else if strings.Contains(finalResponseText, "SHA256 mismatch") {
		fmt.Println("SHA256 mismatch")
	} else {
		fmt.Println(finalResponseText)
	}
	return nil
}

func parsePutCommand(tokens []string) (string, string, error) {
	if len(tokens) < 2 {
		return "", "", fmt.Errorf("usage: put <local_path> [remote_path]")
	}
	if len(tokens) > 3 {
		return "", "", fmt.Errorf("usage: put <local_path> [remote_path]")
	}

	localPath := tokens[1]
	remotePath := ""
	if len(tokens) == 3 {
		remotePath = tokens[2]
	}

	return localPath, remotePath, nil
}

func handleGetDir(conn net.Conn, tokens []string) error {
	remoteDir, localDir, err := parseDirCommand(tokens, "getdir <remote_dir> [local_dir]")
	if err != nil {
		return err
	}
	if localDir == "" {
		localDir = directoryBaseName(remoteDir)
	}

	fmt.Printf("Requesting directory: %s\n", remoteDir)
	fmt.Printf("Saving to: %s\n", localDir)

	if err := SendMessage(conn, []byte("getdir "+QuoteArgument(remoteDir))); err != nil {
		return err
	}

	response, err := ReceiveMessage(conn)
	if err != nil {
		return err
	}

	responseText := string(response)
	if strings.HasPrefix(responseText, "ERROR:") {
		fmt.Println(responseText)
		return nil
	}

	responseParts := strings.Fields(responseText)
	if len(responseParts) != 3 || responseParts[0] != "OK" {
		return fmt.Errorf("unexpected response: %s", responseText)
	}

	fileCount, err := strconv.Atoi(responseParts[1])
	if err != nil {
		return fmt.Errorf("invalid file count %q", responseParts[1])
	}
	totalBytes, err := strconv.ParseInt(responseParts[2], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid total size %q", responseParts[2])
	}

	manifestData, err := ReceiveMessage(conn)
	if err != nil {
		return err
	}

	entries, parsedFileCount, parsedTotalBytes, err := ParseManifest(string(manifestData))
	if err != nil {
		return err
	}
	if parsedFileCount != fileCount || parsedTotalBytes != totalBytes {
		return fmt.Errorf("manifest summary mismatch")
	}

	if err := os.MkdirAll(localDir, 0755); err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.Kind != manifestDir {
			continue
		}
		target, err := safeJoin(localDir, entry.Rel)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(target, 0755); err != nil {
			return err
		}
	}

	progress := newProgressPrinter()
	var completedBytes int64
	var fileIndex int
	for _, entry := range entries {
		if entry.Kind != manifestFile {
			continue
		}

		fileIndex++
		fmt.Printf("File %d/%d: %s\n", fileIndex, fileCount, entry.Rel)
		target, err := safeJoin(localDir, entry.Rel)
		if err != nil {
			return err
		}

		received, err := ReceiveFileWithProgress(conn, target, entry.Size, func(current int64, _ int64) {
			progress(completedBytes+current, totalBytes)
		})
		if err != nil {
			return err
		}
		fmt.Println()

		completedBytes += received
		actualHash, err := ComputeSHA256(target)
		if err != nil {
			return err
		}
		if !strings.EqualFold(actualHash, entry.SHA256) {
			fmt.Printf("SHA256 mismatch: %s\n", entry.Rel)
			continue
		}
		fmt.Printf("SHA256 verified: %s\n", entry.Rel)
	}

	if fileCount == 0 {
		progress(0, 0)
		fmt.Println()
	}

	fmt.Printf("Received %d files, %d bytes\n", fileCount, completedBytes)
	return nil
}

func handlePutDir(conn net.Conn, tokens []string) error {
	localDir, remoteDir, err := parseDirCommand(tokens, "putdir <local_dir> [remote_dir]")
	if err != nil {
		return err
	}

	info, err := os.Stat(localDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("local path is not a directory")
	}

	if remoteDir == "" {
		remoteDir = filepath.Base(filepath.Clean(localDir))
	}

	entries, fileCount, totalBytes, err := BuildDirectoryManifest(localDir)
	if err != nil {
		return err
	}

	fmt.Printf("Uploading directory: %s\n", localDir)
	fmt.Printf("Remote directory: %s\n", remoteDir)

	if err := SendMessage(conn, []byte("putdir "+QuoteArgument(remoteDir))); err != nil {
		return err
	}

	response, err := ReceiveMessage(conn)
	if err != nil {
		return err
	}

	responseText := string(response)
	if responseText != "READY" {
		fmt.Println(responseText)
		return nil
	}

	if err := SendMessage(conn, []byte(SerializeManifest(entries))); err != nil {
		return err
	}

	progress := newProgressPrinter()
	var completedBytes int64
	var fileIndex int
	for _, entry := range entries {
		if entry.Kind != manifestFile {
			continue
		}

		fileIndex++
		fmt.Printf("File %d/%d: %s\n", fileIndex, fileCount, entry.Rel)
		source, err := safeJoin(localDir, entry.Rel)
		if err != nil {
			return err
		}
		if err := SendFileWithProgress(conn, source, func(current int64, _ int64) {
			progress(completedBytes+current, totalBytes)
		}); err != nil {
			return err
		}
		fmt.Println()
		completedBytes += entry.Size
	}

	if fileCount == 0 {
		progress(0, 0)
		fmt.Println()
	}

	finalResponse, err := ReceiveMessage(conn)
	if err != nil {
		return err
	}

	fmt.Println(string(finalResponse))
	return nil
}

func parseDirCommand(tokens []string, usage string) (string, string, error) {
	if len(tokens) < 2 || len(tokens) > 3 {
		return "", "", fmt.Errorf("usage: %s", usage)
	}

	target := ""
	if len(tokens) == 3 {
		target = tokens[2]
	}

	return tokens[1], target, nil
}

func printHelp() {
	fmt.Println(`Commands:
ping
pwd
ls [path]
cd <path>
mkdir <path>
get <remote_path> [local_path]
put <local_path> [remote_path]
getdir <remote_dir> [local_dir]
putdir <local_dir> [remote_dir]
hash <path>
stat <path>
exit

Examples:
ls "C:\Program Files"
get "C:\Users\Public\file one.txt" "./loot/file one.txt"
put "./local file.txt" "C:\Users\Public\remote file.txt"
getdir "C:\Users\Public\Test Folder" "./loot/Test Folder"
putdir "./loot/Test Folder" "C:\Users\Public\Restored Folder"`)
}

func resolveRemotePutPath(localPath, remotePath string) string {
	localName := filepath.Base(filepath.Clean(localPath))
	if remotePath == "" {
		return localName
	}

	if strings.HasSuffix(remotePath, "/") || strings.HasSuffix(remotePath, `\`) {
		return remotePath + localName
	}

	return remotePath
}

func resolveLocalGetPath(remotePath, localPath string) (string, error) {
	remoteName := remoteBaseName(remotePath)
	if remoteName == "." || remoteName == string(filepath.Separator) || remoteName == "" {
		return "", fmt.Errorf("could not determine output filename")
	}

	if localPath == "" {
		return remoteName, nil
	}

	info, err := os.Stat(localPath)
	if err == nil && info.IsDir() {
		return filepath.Join(localPath, remoteName), nil
	}
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	return localPath, nil
}

func remoteBaseName(path string) string {
	path = strings.TrimRight(path, `/\`)
	if path == "" {
		return ""
	}

	slash := strings.LastIndex(path, "/")
	backslash := strings.LastIndex(path, `\`)
	index := slash
	if backslash > index {
		index = backslash
	}
	if index == -1 {
		return path
	}

	return path[index+1:]
}

func newProgressPrinter() func(current int64, total int64) {
	var lastPrint time.Time
	var lastCurrent int64 = -1

	return func(current int64, total int64) {
		now := time.Now()
		if current == 0 && total > 0 && lastPrint.IsZero() {
			return
		}
		if current != total && !lastPrint.IsZero() && now.Sub(lastPrint) < time.Second {
			return
		}
		if current == lastCurrent && current != total {
			return
		}

		percent := 100.0
		if total > 0 {
			percent = float64(current) / float64(total) * 100
		}

		fmt.Printf("\r%.1f%%  %s / %s", percent, FormatBytes(current), FormatBytes(total))
		lastPrint = now
		lastCurrent = current
	}
}
