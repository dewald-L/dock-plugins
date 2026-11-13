# dock-plugins

The index of plugins for [dock](https://github.com/dewald-L/dock). dock reads
[`plugins.json`](plugins.json) to list plugins (`dock plugin search`, the
`Available` section of the Plugins screen) and to install them by id:

```bash
dock plugin search
dock plugin install lazy1on1
dock plugin install --defaults   # what install.sh does
```

## Plugins

| id | kind | default | description |
|---|---|---|---|
| lazy1on1 | native | yes | Continuous 1:1 notes: people, meetings, goals, action items ([repo](https://github.com/dewald-L/lazy1on1)) |
| lazysheet | native | no | Timesheets: log, browse, edit and total osheet entries; uses osheet's config ([repo](https://github.com/dewald-L/lazysheet)) |

`default` plugins are installed by dock's `install.sh` when it gets no
arguments.

## Adding a plugin

1. Make the plugin work with dock (see dock's
   [docs/PLUGINS.md](https://github.com/dewald-L/dock/blob/main/docs/PLUGINS.md)) and
   check `dock plugin install <package>` builds it.
2. Tag a release in the plugin's repo (`git tag v0.1.0 && git push --tags`), unless
   the entry should follow the latest commit.
3. Open a PR here that adds an entry to `plugins.json` and a row to the table above.
   CI runs `go test ./...`.

### Fields

| Field | Required | Meaning |
|---|---|---|
| `id` | yes | The plugin's manifest `ID`: lowercase letters, digits and `-`. Unique. |
| `name` | yes | The manifest `Name`. |
| `description` | yes | The manifest `Description`: one line, at most 120 characters. |
| `kind` | yes | `native` (a Go package dock compiles in) or `exec` (a program on `PATH`). |
| `package` | native | The Go import path of the package that calls `plugin.Register`. |
| `version` | no | native: a tag or commit to pin. Empty means latest. |
| `command` | exec | The argv dock runs, e.g. `["osheet"]`. |
| `summary` | no | exec: argv whose output (3 lines) shows on dock's home screen. |
| `requires` | no | A one-line hint shown on install, e.g. `needs osheet on PATH`. dock registers exec plugins but can't install their program. |
| `repo` | no | An https link to the source. |
| `default` | no | `true`: `install.sh` installs it. |
| `tags` | no | Lowercase words for `dock plugin search`. |

`id`, `name`, `description` and `package` must match the plugin's manifest and
import path.

### What CI checks

- `plugins.json` is valid JSON with `"version": 1`, and has no unknown fields
  (typos). dock itself ignores unknown fields, so new optional fields can be added;
  add them to `validate.go` in the same PR.
- Every entry has a valid, unique id, a name, a one-line description and a known
  kind. Native entries have a package (no `@version` in it); exec entries have a
  command.
- Each native package resolves: `go list -m` finds its module at the pinned version
  (or latest), and the package directory exists in that module.
- This README has a table row for every id.

Run the same checks locally with `go test ./...` (`go test -short ./...` skips the
network). `testdata/bad/` holds broken indexes the checks must reject.

### Format versions

`"version"` is the format of the file, not of any plugin. dock refuses an index with
a newer format and tells the user to update dock. Only bump it for a change old docks
would misread.
