# git-go

A Git implementation written in Go

**Note**: This is pretty much WIP and some things may be broken in some cases and code also comments are missing. These will be worked on when the implementation is stabilized (or if).

## Installation

### Download binary
Download the latest binary from the [GitHub releases page](https://github.com/unkn0wn-root/git-go/releases)

### Build from Source

### Prerequisites
- Go 1.23 or later

```bash
git clone https://github.com/unkn0wn-root/git-go.git
cd git-go
go build -o git-go
```

## Usage

```bash
# Initialize a new repository
./git-go init [directory]

# Check repository status
./git-go status
```

### Staging and Committing
```bash
# Add files to staging area
./git-go add <file>...
./git-go add .                    # Add all files
./git-go add src/                 # Add directory recursively

# Create commit
./git-go commit -m "Commit message"
./git-go commit -m "Message" --author-name "Name" --author-email "hello@local.repo"
```

### History and Inspection
```bash
# View commit history
./git-go log
./git-go log --oneline            # Condensed format
./git-go log --max-count 10       # Limit to 10 commits
./git-go log -n 5                 # Limit to 5 commits

# Show differences
./git-go diff                     # Working tree vs staging area
./git-go diff --staged            # Staging area vs last commit
./git-go diff --cached            # Alternative to --staged
./git-go diff file.txt            # Specific file differences

# Line-by-line authorship
./git-go blame <file>
```

### Reset Operations
```bash
# Reset modes
./git-go reset                    # Mixed reset to HEAD
./git-go reset --soft <commit>    # Move HEAD only
./git-go reset --mixed <commit>   # Move HEAD and reset index
./git-go reset --hard <commit>    # Move HEAD, reset index and working tree

# Path-specific reset
./git-go reset <commit> -- <file>...
./git-go reset HEAD -- file.txt
```

### Remote Operations (Protocol Client)
```bash
# Remote management
./git-go remote add origin <url>
./git-go remote list
./git-go remote show origin

# Clone repository
./git-go clone <url> [directory]

# Pull/Push operations
./git-go pull [remote] [branch]
./git-go push [remote] [branch]
```

## Testing

```bash
go test ./...

# with coverage
go test -cover ./...
```

## Compatibility

git-go tries to maintain full compatibility with standard Git repositories:
- Objects created by git-go can be read by Git
- Repositories initialized by git-go work with Git commands
- Index files are fully compatible between implementations
- Reference structure follows Git conventions
