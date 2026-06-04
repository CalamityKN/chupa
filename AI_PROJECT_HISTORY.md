# AI Project History: chupa

This file is intended for future AI assistants and maintainers working on this repository.
Use it to understand what has already been built, what constraints must be preserved, and how to record future version changes.

## Current Version

- App name: `chupa`
- Version: `0.1.0`
- Repository: `https://github.com/CalamityKN/chupa`
- Initial public release commit: `a3e75f1 Initial chupa release`
- Initial release date: June 4, 2026

## Project Purpose

`chupa` is a lightweight cross-platform TCP file-transfer utility for authorized lab, CTF, and administrative environments.

It is intentionally scoped as a file-transfer-only tool.

It does not:

- execute commands
- spawn shells
- install services
- create persistence
- auto-reconnect
- proxy traffic
- provide SOCKS or port-forwarding behavior
- hide files, windows, or processes
- modify system settings

Preserve this boundary unless the project owner explicitly changes the scope.

## Implemented Functionality Through v0.1.0

### Phase 1: Initial Skeleton

- Created the Go project structure.
- Implemented CLI parsing:
  - `chupa server <port>`
  - `chupa <ip> <port>`
- Added empty server/client stubs.
- Kept code split across:
  - `main.go`
  - `server.go`
  - `client.go`
  - `protocol.go`

### Phase 2: TCP Connectivity and Interactive Session

- Server listens on a TCP port.
- Client connects to server.
- Server accepts one client at a time.
- Server prints connected client address.
- Server becomes interactive controller with prompt:
  - `tool>`
- Added length-prefixed message protocol:
  - `uint32 length`
  - `[]byte payload`
- Added:
  - `SendMessage(conn net.Conn, data []byte) error`
  - `ReceiveMessage(conn net.Conn) ([]byte, error)`
- Added `ping`/`pong`.
- Added `exit`, returning server to wait for a new connection.

### Phase 3: Remote Filesystem Navigation

Client-side filesystem commands:

- `pwd`
- `cd <path>`
- `ls [path]`
- `mkdir <path>`

Behavior:

- Uses Go standard library filesystem APIs only.
- Does not execute shell commands.
- Supports relative and absolute paths.
- Supports Windows drive-root normalization for inputs like `C:`.

### Phase 4: GET File Transfer

Added:

- `get <remote_path>`
- `get <remote_path> <local_path>`

Behavior:

- Server requests a file from the client.
- Client validates the source file.
- Client sends metadata and streams raw file bytes.
- Server writes the file locally.
- Parent directories are created as needed.
- File contents are streamed, not loaded fully into memory.

### Phase 5: PUT File Transfer

Added:

- `put <local_path>`
- `put <local_path> <remote_path>`

Behavior:

- Server validates the local file before upload.
- Client receives and writes file bytes.
- Client creates parent directories as needed.
- Transfer uses a `READY` handshake before streaming bytes.

### Phase 6: SHA256 Verification

Added:

- `ComputeSHA256(path string) (string, error)`
- `hash <path>`
- `stat <path>`

Behavior:

- GET metadata includes SHA256.
- PUT metadata includes SHA256.
- Receivers compute SHA256 after writing.
- Output reports:
  - `SHA256 verified`
  - or `SHA256 mismatch`
- Hashing is streamed through `crypto/sha256`.

### Phase 7: Quoted Path Parsing and Help

Added:

- `ParseCommandLine(input string) ([]string, error)`
- `QuoteArgument(value string) string`
- `help`

Behavior:

- Supports double-quoted paths with spaces.
- Supports quoted paths for:
  - `cd`
  - `ls`
  - `mkdir`
  - `get`
  - `put`
  - `hash`
  - `stat`
- Unmatched quotes return a clear error without breaking the session.
- `help` is handled server-side and is not sent to the client.

### Phase 8: Transfer Progress Output

Added:

- `FormatBytes(n int64) string`
- `SendFileWithProgress(...)`
- `ReceiveFileWithProgress(...)`

Behavior:

- Server/controller shows progress for GET and PUT.
- Progress format:
  - `100.0%  10.00 MB / 10.00 MB`
- Progress updates are throttled to about once per second and always show completion.
- Existing non-progress transfer helpers remain as wrappers.

### Phase 9: Recursive Directory Transfer

Added:

- `getdir <remote_dir> [local_dir]`
- `putdir <local_dir> [remote_dir]`
- `dirtransfer.go`

Manifest format:

```text
D|relative/path
F|relative/path|size|sha256
```

Behavior:

- Recursively transfers directory contents.
- Preserves relative paths.
- Supports nested directories and filenames with spaces.
- Supports empty directories.
- Uses `/` in manifest paths across platforms.
- Verifies SHA256 per file.
- Shows aggregate directory-transfer progress.

Safety:

- Manifest paths reject:
  - absolute paths
  - drive-letter paths
  - `..` path traversal
  - paths escaping the destination root
- Symlinks and special files are skipped.

### Phase 10: Release Packaging, Version Output, and README

Added:

- App constants:
  - `const AppName = "chupa"`
  - `const Version = "0.1.0"`
- Version commands:
  - `chupa version`
  - `chupa --version`
  - `chupa -v`
- `build.ps1`
- `.gitignore`
- `README.md`
- helper tests:
  - `main_test.go`
  - `dirtransfer_test.go`

Release artifacts built by `build.ps1`:

```text
dist\chupa-windows-amd64.exe
dist\chupa-linux-amd64
dist\chupa-linux-arm64
```

Tests cover:

- quoted command parsing
- unmatched quote handling
- byte formatting
- manifest path validation

## Current Command Set

```text
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
help
exit
```

## Important Files

- `main.go`
  - CLI parsing
  - version handling
  - shared command-line parser
- `server.go`
  - TCP listener
  - interactive prompt
  - server-side command dispatch
  - GET/PUT/GETDIR/PUTDIR controller behavior
  - progress printing
- `client.go`
  - TCP client connection
  - client-side command handling
  - filesystem operations
  - file and directory receive/send handlers
- `protocol.go`
  - length-prefixed message protocol
  - file streaming helpers
  - SHA256 helper
  - byte formatting
- `dirtransfer.go`
  - recursive directory manifest build/parse logic
  - manifest safety validation
  - safe destination path joining
- `build.ps1`
  - gofmt
  - tests
  - cross-compilation
- `README.md`
  - user-facing documentation
- `main_test.go`
  - parser and byte-format tests
- `dirtransfer_test.go`
  - manifest path-safety tests

## Verification Commands

Use these before release or after meaningful changes:

```powershell
gofmt -w *.go
go test ./...
go build -o chupa.exe .
.\chupa.exe version
.\build.ps1
```

Expected version output:

```text
chupa 0.1.0
```

## Build Notes

The local environment used during development had Go installed at:

```text
C:\Program Files\Go\bin
```

`build.ps1` first checks PATH and then falls back to that location.

## Version Change Log

### v0.1.0

Initial public release.

Included:

- TCP server/client modes
- interactive controller prompt
- length-prefixed messaging
- remote filesystem navigation
- streamed GET/PUT
- recursive GETDIR/PUTDIR
- SHA256 integrity verification
- quoted path parsing
- progress output
- help output
- version output
- README
- PowerShell release build script
- helper tests

## Future Version Tracking Instructions

When adding functionality:

1. Update `Version` in `main.go`.
2. Add a new section under `Version Change Log`.
3. Record:
   - new commands
   - changed protocol behavior
   - compatibility impact
   - new safety constraints
   - new tests
   - verification commands run
4. Update `README.md`.
5. Update `help` output in `server.go` if commands change.
6. Add or update tests for pure helper functions.
7. Run:

```powershell
gofmt -w *.go
go test ./...
go build -o chupa.exe .
.\build.ps1
```

## Safety Reminder for Future AI

Do not add command execution, shell access, persistence, service installation, stealth behavior, port forwarding, proxying, or auto-reconnect unless the project owner explicitly redefines the scope.

Default stance: keep `chupa` a transparent, interactive, authorized file-transfer utility.
