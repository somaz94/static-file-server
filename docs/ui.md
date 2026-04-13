# Directory Listing UI

The directory listing features a modern, responsive design with extensive interactive capabilities.

<br/>

## Theme

- **Dark/light mode**: Manual toggle + automatic system preference detection
- Theme preference persisted in localStorage

<br/>

## Views

- **Table view**: Sortable columns (name, size, date) with sticky header
- **Grid view**: Card layout with image thumbnails (`g` key to toggle)
- View preference persisted in localStorage

<br/>

## File Display

- **File icons**: 13 categories with distinct colors (image, video, audio, pdf, doc, sheet, slide, archive, code, config, binary, font, file)
- **Extension badges**: Raw file extension display (`.go`, `.py`, `.tsx`, etc.)
- **Relative time**: "3h ago" format with absolute time tooltip on hover
- **Size formatting**: Human-readable sizes (KB, MB, GB, TB)
- **Breadcrumb navigation**: Clickable path segments with current page indicator

<br/>

## Search & Filter

- **Real-time search**: Instant filtering with text highlight (`/` to focus, `Esc` to clear)
- **Filter chips**: Category filters (All, Folders, Images, Code, etc.) with dynamic count badges
- **URL hash state**: Shareable search/filter links via URL hash

<br/>

## Preview

- **Image preview**: Click to open, zoom (click toggle + scroll wheel), gallery navigation (`←` `→` keys)
- **Video/Audio preview**: Inline media player
- **PDF preview**: Iframe viewer with download button
- **Text/Code preview**: Syntax display with line numbers
- **Download button**: Direct download from preview modal

<br/>

## Selection & Download

- **Multi-select**: Checkbox selection with "Select all" support
- **Batch ZIP download**: Multiple files bundled as ZIP (max 100 files, 500 MB)
- **Single file download**: Direct download without ZIP wrapping
- **Copy URL**: Full URL copied to clipboard (hover to reveal button)

<br/>

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `/` | Focus search |
| `Esc` | Clear search / close preview |
| `g` | Toggle grid/list view |
| `←` `→` | Gallery prev/next |
| `↑` `↓` | Navigate rows |
| `Enter` | Open file/directory |
| `Space` | Toggle row selection |
| `?` | Show shortcuts help |

<br/>

## Accessibility

- `<main>` landmark for screen readers
- `aria-label` on breadcrumb and search
- `aria-live` on search result count
- `aria-sort` on sortable table headers
- `role="dialog"` and `aria-modal` on preview overlay
- Focus trap in preview modal
- Full keyboard-only navigation support

<br/>

## Footer

- Total file and directory counts
- Combined size of all entries
- Server version display
