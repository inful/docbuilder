# VS Code Edit Link Integration for Preview Mode

## Overview

When running the `preview` command in VS Code (local or remote), edit links in the documentation site will automatically open files in VS Code for editing instead of navigating to an external forge.

## How It Works

1. **Edit URL Detection**: The `VSCodeDetector` identifies when running in local preview mode (repository name is "local") and generates special `/_edit/<filepath>` URLs instead of forge URLs.

2. **HTTP Handler**: The preview server registers a `/_edit/` endpoint that:
   - Extracts the file path from the URL
   - Validates the path (prevents directory traversal attacks)
   - Executes `code <filepath>` to open the file in VS Code
   - Redirects back to the page you were viewing

3. **Cross-Environment Support**: Works in both:
   - **Local VS Code**: Opens files in your local editor
   - **VS Code Remote/Dev Containers**: The `code` command works seamlessly in remote sessions

## Usage

Simply run the preview command:

```bash
docbuilder preview --docs-dir ./docs
```

Then navigate to the documentation site at `http://localhost:1316`. All "Edit this page" links will now open files directly in VS Code.

## Implementation Details

### Components

1. **VSCodeDetector** (`internal/hugo/editlink/vscode_detector.go`)
   - Implements the `ForgeDetector` interface
   - Activated only when repository name is "local"
   - Returns special "vscode" forge type

2. **StandardEditURLBuilder** (`internal/hugo/editlink/url_builder.go`)
   - Handles "vscode" forge type
   - Generates `/_edit/<filepath>` URLs

3. **HTTP Handler** (`internal/daemon/vscode_edit_handler.go`)
   - Registered at `/_edit/` endpoint
   - Validates paths against docs directory
   - Executes `code` command with timeout
   - Redirects to referer

### Security

- **Path Validation**: Ensures requested paths are within the docs directory
- **Path Traversal Prevention**: Uses `filepath.Clean()` and prefix checking
- **Command Timeout**: 5-second timeout on `code` command execution
- **Context Awareness**: Uses request context for cancellation

### Testing

Tests verify:
- VS Code detector activates only for "local" repositories
- URL builder generates correct `/_edit/` URLs
- Non-local repositories continue using forge URLs

Run tests:
```bash
go test ./internal/hugo/editlink/ -run TestVSCode
```

## Example Flow

1. User navigates to `http://localhost:1316/how-to/release-process/`
2. User clicks "Edit this page" link: `http://localhost:1316/_edit/how-to/release-process.md`
3. Server receives request at `/_edit/` handler
4. Handler validates path: `/workspaces/docbuilder/docs/how-to/release-process.md`
5. Handler executes: `code /workspaces/docbuilder/docs/how-to/release-process.md`
6. VS Code opens the file for editing
7. Browser redirects back to `http://localhost:1316/how-to/release-process/`
8. User edits file in VS Code
9. File watcher detects change and rebuilds site automatically

## Future Enhancements

Potential improvements:
- Add `--edit-line <number>` support to open files at specific lines
- Support other editors via configuration (`vim`, `emacs`, etc.)
- Add visual feedback in the UI when file opens successfully
- Cache file path mappings for faster lookups
