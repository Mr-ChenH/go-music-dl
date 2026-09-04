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

## Development

With [mise](https://mise.jdx.dev/) installed, run the complete Web development stack from the repository root:

```bash
mise install
mise run setup
mise run dev
```

Open `http://localhost:8080/music/`. The frontend task continuously rebuilds the React assets, and the backend task restarts the Go server when source files, templates, or embedded assets change. Run either side separately with `mise run frontend` or `mise run backend`.
