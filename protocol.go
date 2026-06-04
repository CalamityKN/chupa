package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
)

func SendMessage(conn net.Conn, data []byte) error {
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(data)))

	if err := writeAll(conn, header); err != nil {
		return err
	}

	return writeAll(conn, data)
}

func ReceiveMessage(conn net.Conn) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(header)
	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, err
	}

	return data, nil
}

func SendFile(conn net.Conn, path string) error {
	return SendFileWithProgress(conn, path, nil)
}

func SendFileWithProgress(conn net.Conn, path string, onProgress func(sent int64, total int64)) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	_, err = copyWithProgress(conn, file, info.Size(), onProgress)
	return err
}

func ReceiveFile(conn net.Conn, outputPath string, size int64) (int64, error) {
	return ReceiveFileWithProgress(conn, outputPath, size, nil)
}

func ReceiveFileWithProgress(conn net.Conn, outputPath string, size int64, onProgress func(received int64, total int64)) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return 0, err
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	copied, err := copyWithProgress(file, io.LimitReader(conn, size), size, onProgress)
	if err != nil {
		return copied, err
	}
	if copied != size {
		return copied, io.ErrUnexpectedEOF
	}

	return copied, nil
}

func ComputeSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func FormatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}

	value := float64(n)
	units := []string{"KB", "MB", "GB", "TB"}
	for _, suffix := range units {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.2f %s", value, suffix)
		}
	}

	return fmt.Sprintf("%.2f PB", value/unit)
}

func copyWithProgress(dst io.Writer, src io.Reader, total int64, onProgress func(current int64, total int64)) (int64, error) {
	buffer := make([]byte, 64*1024)
	var copied int64

	if onProgress != nil {
		onProgress(0, total)
	}

	for {
		nr, readErr := src.Read(buffer)
		if nr > 0 {
			nw, writeErr := dst.Write(buffer[:nr])
			if nw > 0 {
				copied += int64(nw)
				if onProgress != nil {
					onProgress(copied, total)
				}
			}
			if writeErr != nil {
				return copied, writeErr
			}
			if nw != nr {
				return copied, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return copied, readErr
		}
	}

	return copied, nil
}

func writeAll(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrUnexpectedEOF
		}
		data = data[n:]
	}

	return nil
}
