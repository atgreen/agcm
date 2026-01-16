# agcm

A terminal user interface (TUI) for the Red Hat Customer Portal API.

![License](https://img.shields.io/badge/license-GPL--3.0--or--later-blue)

## Features

- **Case Management** - Browse and view Red Hat support cases with a keyboard-driven interface
- **Case Details** - View case descriptions, comments, and attachments in tabbed panels
- **Filtering** - Filter cases by status, severity, product, keyword, and account(s)
- **Filter Presets** - Save and recall up to 10 filter combinations with hotkeys
- **Sorting** - Sort cases by last modified date, created date, severity, or case number
- **Quick Search** - Jump directly to a case by number with `/`
- **Text Search** - Search within case content with `Ctrl+F`
- **Export** - Export individual cases or bulk export all cases to markdown
- **Mouse Support** - Click to select cases, scroll, switch tabs, and open links
- **Cross-Platform** - Builds for Linux, macOS, and Windows

## Installation

Download the latest release for your platform from the [releases page](https://github.com/atgreen/agcm/releases).

Or build from source:

```bash
git clone https://github.com/atgreen/agcm.git
cd agcm
make build
```

## Configuration

### Authentication

agcm uses offline tokens for authentication with the Red Hat Customer Portal API.

1. Obtain an offline token from https://access.redhat.com/management/api
2. Run `agcm auth login` and paste your token when prompted
3. Start the TUI with `agcm`

### Token Storage

Tokens are stored securely using your system's native credential manager:

| OS | Storage Location |
|----|------------------|
| **macOS** | Keychain (Keychain Access app) |
| **Windows** | Credential Manager |
| **Linux** | Secret Service API (GNOME Keyring / KDE Wallet) |

On headless systems where no keyring is available, tokens fall back to file-based storage in the config directory.

Use `agcm auth status` to check which storage method is being used.

### Config File

Configuration is stored at `~/.config/agcm/config.yaml`:

```yaml
api:
  base_url: https://api.access.redhat.com
defaults:
  account_number: ""    # Default account filter
  group_number: ""      # Default group filter
presets:                # Filter presets (keys 1-9, 0)
  "1":
    name: "Team A Critical"
    accounts: ["123456", "789012"]
    status: ["Open", "Waiting on Red Hat"]
    severity: ["1 (Urgent)", "2 (High)"]
  "2":
    name: "All Open"
    status: ["Open"]
```

## Usage

```bash
agcm                          # Launch the TUI
agcm -a 12345                 # Filter by account number
agcm -a 12345,67890           # Filter by multiple accounts
agcm -p 1                     # Load filter preset 1
agcm --group 67890            # Filter by case group
agcm --mask                   # Mask sensitive text for screenshots
agcm --version                # Show version
agcm --help                   # Show help
```

### CLI Commands

In addition to the TUI, agcm provides command-line tools for scripting and automation:

#### List Cases

```bash
agcm list cases                     # List all cases
agcm list cases --status open       # Filter by status
agcm list cases --severity 1        # Filter by severity
agcm list cases --limit 50          # Limit results
agcm list accounts                  # List accessible accounts
```

#### Show Case Details

```bash
agcm show case 01234567             # Show case details in markdown
agcm show case 01234567 --comments  # Include comments (default)
```

#### Export to Markdown

```bash
agcm export case 01234567           # Export single case
agcm export case 01234567 01234568  # Export multiple cases
agcm export cases --output ./cases  # Export all cases to directory
agcm export cases --status open     # Export filtered cases
```

#### Search

```bash
agcm search "kernel panic"          # Search cases and solutions
agcm search "NVMe driver" --limit 20
```

#### Authentication & Updates

```bash
agcm auth login         # Authenticate with offline token
agcm auth logout        # Remove stored credentials
agcm auth status        # Check authentication status
agcm update             # Update to latest version
agcm update --check     # Check for updates without installing
```

## Keyboard Shortcuts (TUI)

| Key | Action |
|-----|--------|
| `↑/k`, `↓/j` | Navigate up/down |
| `←/→` | Switch detail tabs |
| `gg`, `G` | Go to top/bottom |
| `PgUp/PgDn` | Page up/down |
| `Tab` | Switch between list and detail panes |
| `Esc` | Back to list |
| `/` | Quick search by case number |
| `f` | Filter dialog |
| `F` | Clear filter |
| `1-9`, `0` | Load filter preset |
| `Ctrl+s` | Save current filter to preset (then press 1-9/0) |
| `Ctrl+F` | Search within case |
| `n`, `p` | Next/previous comment (Comments tab) |
| `s` | Cycle sort field |
| `S` | Toggle sort order |
| `r` | Refresh |
| `e` | Export current case |
| `E` | Export all cases |
| `?` | Toggle help |
| `q` | Quit |

### Mouse

- **Left-click** - Select case, switch tabs, click links
- **Right-click** - Open case in browser
- **Scroll** - Navigate lists and detail content
- **Drag scrollbar** - Quick scroll

## Building

```bash
make build          # Build for current platform
make build-all      # Build for all platforms
make release        # Create release archives with checksums
make test           # Run tests
make lint           # Run linter
```

## Author

Anthony Green

## License

This project is licensed under the GNU General Public License v3.0 or later - see the [LICENSE](LICENSE) file for details.
