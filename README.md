# agcm

A terminal user interface (TUI) for the Red Hat Customer Portal API.

![License](https://img.shields.io/badge/license-GPL--3.0--or--later-blue)

## Features

- **Case Management** - Browse and view Red Hat support cases with a keyboard-driven interface
- **Case Details** - View case descriptions, comments, and attachments in tabbed panels
- **Filtering** - Filter cases by status, severity, product, and keyword
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
```

## Usage

```bash
agcm                    # Launch the TUI
agcm --account 12345    # Filter by account number
agcm --group 67890      # Filter by case group
agcm --mask             # Mask sensitive text for screenshots
agcm --version          # Show version
agcm --help             # Show help
```

### Subcommands

```bash
agcm auth login         # Authenticate with offline token
agcm auth logout        # Remove stored credentials
agcm auth status        # Check authentication status
agcm update             # Update to latest version
agcm update --check     # Check for updates without installing
```

## Keyboard Shortcuts

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
