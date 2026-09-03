# React Web UI

The React layer renders the main search console and global command rail with Fluent 2 components and icons. Existing server-rendered result lists and business dialogs remain in the Go templates while they are migrated incrementally.

## Build

```bash
cd webui
npm install
npm run build
```

Vite writes deterministic assets to `internal/web/templates/static/react/`. These generated files are committed because the Go application embeds them into the release binary.

The server injects page state through `data-*` attributes on the React mount nodes. `window.mountMusicDlReact()` is called after both initial load and the existing Ajax navigation flow.
