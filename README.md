# chupa

Lightweight cross-platform file transfer utility for authorized lab, CTF, and administrative environments.

## Scope

chupa is only a file-transfer utility. It does not execute commands, spawn shells, install services, persist, proxy traffic, or modify system settings.

## Build

Build a Windows binary:

```powershell
go build -o chupa.exe .
```

Build release artifacts:

```powershell
.\build.ps1
```

The release script creates:

```text
dist\chupa-windows-amd64.exe
dist\chupa-linux-amd64
dist\chupa-linux-arm64
```

## Usage

Run the controller/server:

```text
chupa server 1234
```

Run the client:

```text
chupa 10.10.10.10 1234
```

Print version:

```text
chupa version
chupa --version
chupa -v
```

## Commands

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

## Examples

```text
ls "C:\Program Files"
cd "C:\Users\Public\Test Folder"
mkdir "C:\Users\Public\Test Folder"

get "C:\Users\Public\Test Folder\file one.txt"
get "C:\Users\Public\Test Folder\file one.txt" "./loot/file one.txt"

put "./local file.txt" "C:\Users\Public\Test Folder\remote file.txt"

getdir "C:\Users\Public\Test Folder" "./loot/Test Folder"
putdir "./loot/Test Folder" "C:\Users\Public\Restored Folder"

hash "C:\Users\Public\Test Folder\remote file.txt"
stat "C:\Users\Public\Test Folder\remote file.txt"
```

## Safety

Use only in environments you own or are authorized to test.
