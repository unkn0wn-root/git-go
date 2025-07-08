# git-go

A Git implementation written in Go.

**Note**: This is pretty much WIP and some things may be broken in some cases. There are no docs. and code comments are almost non-existing but these will be worked on when the implementation is stabilized (if ever). You're welcome to create PR and fix bugs, add features etc.

**Note 2**: Since I'm operating with Unix style file metadata and I don't really care about Windows implementation right now - **Windows is not supported**.

**Note 3**: Git credential helpers (like `git-credential-store`, `git-credential-osxkeychain`, etc.) are **not yet supported**. You must configure authentication manually (see [here](#authentication)).

## Installation

### Download binary
Download the latest binary from the [GitHub releases page](https://github.com/unkn0wn-root/git-go/releases)

### Build from Source

### Prerequisites
- Go 1.23 or later

```bash
git clone https://github.com/unkn0wn-root/git-go.git
cd git-go
go mod download
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

## Authentication

### GitHub Authentication

For GitHub repositories, use a Personal Access Token (PAT):

```bash
# Set your GitHub token
export GITHUB_TOKEN="your_personal_access_token"

# Then use HTTPS URLs
./git-go clone https://github.com/username/repo.git
./git-go push origin main
```

### GitLab Authentication

For GitLab repositories, use a Personal Access Token:

```bash
# Set your GitLab token
export GITLAB_TOKEN="your_personal_access_token"

# Then use HTTPS URLs
./git-go clone https://gitlab.com/username/repo.git
./git-go push origin main
```

### SSH Authentication

SSH authentication is supported using:

1. **SSH Agent** (recommended):
   ```bash
   # Start ssh-agent and add your key
   eval $(ssh-agent -s)
   ssh-add ~/.ssh/id_rsa

   # Use SSH URLs
   ./git-go clone git@github.com:username/repo.git
   ```

2. **Direct SSH Key**:
   ```bash
   # Will automatically discover keys in ~/.ssh/
   # Supports: id_rsa, id_ed25519, id_ecdsa
   ./git-go clone git@github.com:username/repo.git
   ```

### Basic HTTP Authentication

For repositories requiring username/password:

```bash
export GIT_USERNAME="your_username"
export GIT_PASSWORD="your_password"

./git-go clone https://superdomain.lucky/repo.git
```

### Creating Personal Access Tokens

**GitHub**:
1. Go to GitHub Settings → Developer settings → Personal access tokens
2. Generate new token with needed permissions (repo, etc.)
3. Copy the token and set it as `GITHUB_TOKEN`

**GitLab**:
1. Go to GitLab User Settings → Access Tokens
2. Create new token with scopes (read_repository, write_repository, etc.)
3. Copy the token and set it as `GITLAB_TOKEN`

## Testing

```bash
go test ./...

# with coverage
go test -cover ./...
```

## Project Structure

```
git-go/
├── cmd/                   # Command-line interface definitions
│   ├── add.go             # Add command implementation
│   ├── blame.go           # Blame command implementation
│   ├── clone.go           # Clone command implementation
│   ├── commit.go          # Commit command implementation
│   ├── diff.go            # Diff command implementation
│   ├── init.go            # Init command implementation
│   ├── log.go             # Log command implementation
│   ├── pull.go            # Pull command implementation
│   ├── push.go            # Push command implementation
│   ├── remote.go          # Remote command implementation
│   ├── reset.go           # Reset command implementation
│   ├── root.go            # Root command and CLI setup
│   └── status.go          # Status command implementation
├── internal/              # Internal packages (not exposed to external consumers)
│   ├── commands/          # Command implementations
│   │   ├── add/           # Add command logic and tests
│   │   ├── blame/         # Blame command logic and tests
│   │   ├── clone/         # Clone command logic and tests
│   │   ├── commit/        # Commit command logic and tests
│   │   ├── diff/          # Diff command logic and tests
│   │   ├── log/           # Log command logic and tests
│   │   ├── reset/         # Reset command logic and tests
│   │   └── status/        # Status command logic and tests
│   ├── core/              # Core Git functionality
│   │   ├── discovery/     # Repository discovery utilities
│   │   ├── gitignore/     # .gitignore file parsing and matching
│   │   ├── hash/          # SHA-1 hashing utilities
│   │   ├── index/         # Git index (staging area) operations
│   │   ├── objects/       # Git object parsing and manipulation
│   │   ├── pack/          # Git pack file handling
│   │   └── repository/    # Repository initialization and management
│   └── transport/         # Network transport layer
│       ├── pull/          # Pull operation implementation
│       ├── push/          # Push operation implementation
│       ├── remote/        # Remote repository management
│       └── ssh/           # SSH authentication and transport
├── pkg/                   # Public packages (can be imported by external code)
│   ├── display/           # Output formatting and display utilities
│   │   ├── command.go     # Command output formatting
│   │   ├── diff.go        # Diff output formatting
│   │   ├── display.go     # General display utilities
│   │   ├── log.go         # Log output formatting
│   │   └── status.go      # Status output formatting
│   └── errors/            # Error handling and custom error types
├── main.go                # Application entry point
├── go.mod                 # Go module definition
├── go.sum                 # Go module checksums
├── Makefile               # Build and development tasks
├── LICENSE                # Project license
└── README.md              # Project documentation
```

## Compatibility

git-go tries to maintain full compatibility with standard Git repositories:
- Objects created by git-go can be read by Git
- Repositories initialized by git-go work with Git commands
- Index files are fully (or should be) compatible between implementations
- Reference structure follows Git conventions
