package main

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	manifestDir  = "D"
	manifestFile = "F"
)

type manifestEntry struct {
	Kind   string
	Rel    string
	Size   int64
	SHA256 string
}

func BuildDirectoryManifest(root string) ([]manifestEntry, int, int64, error) {
	var entries []manifestEntry
	var fileCount int
	var totalBytes int64

	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if current == root {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		if info.Mode()&fs.ModeSymlink != 0 {
			return nil
		}

		rel, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}

		manifestRel, err := normalizeManifestRel(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		if strings.ContainsAny(manifestRel, "|\r\n") {
			return fmt.Errorf("unsupported character in path: %s", manifestRel)
		}

		if entry.IsDir() {
			entries = append(entries, manifestEntry{Kind: manifestDir, Rel: manifestRel})
			return nil
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		hash, err := ComputeSHA256(current)
		if err != nil {
			return err
		}

		entries = append(entries, manifestEntry{
			Kind:   manifestFile,
			Rel:    manifestRel,
			Size:   info.Size(),
			SHA256: hash,
		})
		fileCount++
		totalBytes += info.Size()
		return nil
	})
	if err != nil {
		return nil, 0, 0, err
	}

	return entries, fileCount, totalBytes, nil
}

func SerializeManifest(entries []manifestEntry) string {
	var output strings.Builder

	for _, entry := range entries {
		switch entry.Kind {
		case manifestDir:
			fmt.Fprintf(&output, "D|%s\n", entry.Rel)
		case manifestFile:
			fmt.Fprintf(&output, "F|%s|%d|%s\n", entry.Rel, entry.Size, entry.SHA256)
		}
	}

	return output.String()
}

func ParseManifest(text string) ([]manifestEntry, int, int64, error) {
	var entries []manifestEntry
	var fileCount int
	var totalBytes int64

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		switch {
		case len(parts) == 2 && parts[0] == manifestDir:
			rel, err := normalizeManifestRel(parts[1])
			if err != nil {
				return nil, 0, 0, err
			}
			entries = append(entries, manifestEntry{Kind: manifestDir, Rel: rel})
		case len(parts) == 4 && parts[0] == manifestFile:
			rel, err := normalizeManifestRel(parts[1])
			if err != nil {
				return nil, 0, 0, err
			}

			size, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("invalid manifest size for %s", rel)
			}
			if size < 0 {
				return nil, 0, 0, fmt.Errorf("negative manifest size for %s", rel)
			}

			entries = append(entries, manifestEntry{
				Kind:   manifestFile,
				Rel:    rel,
				Size:   size,
				SHA256: parts[3],
			})
			fileCount++
			totalBytes += size
		default:
			return nil, 0, 0, fmt.Errorf("invalid manifest line: %s", line)
		}
	}

	return entries, fileCount, totalBytes, nil
}

func safeJoin(root, rel string) (string, error) {
	rel, err := normalizeManifestRel(rel)
	if err != nil {
		return "", err
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	target := filepath.Join(rootAbs, filepath.FromSlash(rel))
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}

	if targetAbs != rootAbs && !strings.HasPrefix(targetAbs, rootAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes destination: %s", rel)
	}

	return targetAbs, nil
}

func normalizeManifestRel(rel string) (string, error) {
	rel = strings.ReplaceAll(rel, `\`, `/`)
	rel = path.Clean(rel)

	if rel == "." || rel == "" {
		return "", fmt.Errorf("empty relative path")
	}
	if strings.HasPrefix(rel, "/") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute manifest path rejected: %s", rel)
	}
	if len(rel) >= 2 && rel[1] == ':' && isAsciiLetter(rel[0]) {
		return "", fmt.Errorf("drive-letter manifest path rejected: %s", rel)
	}

	for _, part := range strings.Split(rel, "/") {
		if part == ".." {
			return "", fmt.Errorf("path traversal rejected: %s", rel)
		}
	}

	return rel, nil
}

func directoryBaseName(value string) string {
	value = strings.TrimRight(value, `/\`)
	if value == "" {
		return "download"
	}

	name := remoteBaseName(value)
	if name == "" || name == "." {
		return "download"
	}
	return name
}
