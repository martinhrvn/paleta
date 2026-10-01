# `.pltrc` reference

`.pltrc` is the YAML file that tells paleta (`plt`) which folders of a repository
to offer and which commands to run in each. `plt init` writes one for you. This
page documents every key it understands.

- [Quick reference](#quick-reference)
- [Where plt finds the config](#where-plt-finds-the-config)
- [Top-level keys](#top-level-keys)
- [Location keys](#location-keys)
- [Project types](#project-types)
- [Glob locations](#glob-locations)
- [Commands](#commands)
- [Command references (`@project:command`)](#command-references)
- [Environment variables](#environment-variables)
- [Command filtering](#command-filtering)
- [Focus](#focus)
- [Frecency](#frecency)
- [Tools](#tools)
- [Custom project types](#custom-project-types)
- [Global project configuration](#global-project-configuration)
- [Checking your edits](#checking-your-edits)
- [Examples](#examples)

## Quick reference

Every key, annotated. Only `locations` and each location's `location` are
required.

```yaml
locations:                      # required: the folders plt offers
  - name: "web"                 # optional: display name, and the @web:… reference name
    location: "packages/web"    # required: path relative to the .pltrc (a trailing /* is a glob)
    type: "pnpm"                # optional: a type or a list [npm, docker]; omit to detect, `none` to opt out
    env:                        # optional: env vars for every command in this location
      NODE_ENV: "development"
    commands:                   # optional: commands added to the ones the type discovers
      - "npm run legacy"        # string form
      - name: "build"           # object form (recommended)
        command: "pnpm build"
      - name: "ci"
        command:                # a list is joined with &&
          - pnpm install
          - "@api:test"         # a reference to another location's command
        env:                    # optional: per-command env (wins over location env)
          CI: "true"
    include_commands: ["build*", "ci"]   # optional: allow-list (glob patterns on command names)
    exclude_commands: ["*:watch"]        # optional: deny-list, applied after include

  - location: "services/*"      # glob: one location per folder in services/
    type: "go"
    exclude_locations: ["legacy"]        # glob only: drop folders by name
    overrides:                           # glob only: per-folder tweaks
      api:
        env: { PORT: "8080" }

focused: ["web"]                # optional: default the selector to these locations
tools: [lazygit, docker]        # optional: location-independent tools shown at the end
frecency:                       # optional: how usage history ranks commands
  enabled: true
  recency_weight: 50
  frequency_weight: 50
root: /abs/path                 # global project configs only (see below)
```

## Where plt finds the config

plt looks for `.pltrc` in the current directory, then in each parent directory up
the tree. The first one it finds wins. Relative `location` paths resolve against
the directory that holds that `.pltrc`.

If no `.pltrc` is found, plt falls back to the
[global project configuration](#global-project-configuration).

## Top-level keys

| Key | Type | Purpose |
|---|---|---|
| `locations` | list of [locations](#location-keys) | The folders plt offers, and their commands. |
| `focused` | list of strings | Locations the selector shows by default. See [Focus](#focus). |
| `tools` | list of strings | Tools to enable. See [Tools](#tools). |
| `frecency` | map | Ranking by usage history. See [Frecency](#frecency). |
| `root` | absolute path | Only in global project configs: the project the file applies to. |

## Location keys

| Key | Type | Purpose |
|---|---|---|
| `location` | path | **Required.** The folder, relative to the `.pltrc`. It may end in `/*` (see [Glob locations](#glob-locations)). |
| `name` | string | The display name in the selector, also used in `@name:command` references. Defaults to the folder's name. |
| `type` | string or list | The project type(s) used to discover commands. See [Project types](#project-types). |
| `commands` | list | Commands added on top of the discovered ones. See [Commands](#commands). |
| `env` | map | Environment variables for every command here. See [Environment variables](#environment-variables). |
| `include_commands` | list of patterns | An allow-list of commands. See [Command filtering](#command-filtering). |
| `exclude_commands` | list of patterns | A deny-list of commands, applied after the allow-list. |
| `exclude_locations` | list of patterns | Glob locations only: folders to drop. |
| `overrides` | map of folder pattern → location keys | Glob locations only: per-folder changes. |

Location and command names must use only the characters that can appear in a
reference:

- Location `name`: letters, digits, `.`, `_`, `/` and `-`.
- Command `name`: letters, digits, `.`, `_`, `:` and `-`. It must start with a letter or a digit.

Names with other characters still work. The selector flags them, and
`plt lint --fix` rewrites the offending characters to `_`.

> `include_commands` / `exclude_commands` used to be spelled `include` /
> `exclude`. The old names still work, but `plt lint` reports them and
> `plt lint --fix` renames them in place (your comments are kept).

## Project types

`type` decides which commands plt discovers in a folder:

- `type: npm` sets a single type.
- `type: [npm, docker]` sets several. Each command is then labelled with its type
  in the selector (e.g. `svc: [npm] build` and `svc: [docker] build`), so commands
  with the same name stay distinguishable.
- If you omit `type`, plt detects types from the folder's files, separately for
  each folder of a glob.
- `type: none` turns detection off. Only the commands you wrote remain.

Built-in types:

| Type | Detected by | Commands |
|---|---|---|
| `npm`, `yarn`, `pnpm` | `package.json` (the lockfile picks the manager) | The `scripts` in `package.json`. |
| `go` | `go.mod` | `build`, `test`, `fmt`, `vet`, `mod`, … |
| `rust` | `Cargo.toml` | `build`, `test`, `run`, `check`, `clippy`, … |
| `python` | `pyproject.toml`, `setup.py`, `requirements.txt` | `install`, `test`, `lint`, `format`, and the `[project.scripts]` entries |
| `make` | `Makefile` | The make targets. |
| `gradle` | `build.gradle(.kts)` | The gradle tasks. |
| `maven` | `pom.xml` | The maven lifecycle phases. |
| `docker` | `Dockerfile` | `docker build` / `docker run` |
| `compose` | `docker-compose.yml`, `compose.yml` and `*.{dev,…}.yml` variants | `up` / `down` / `build` / `logs` / `ps`. Each variant file gets its own set too: `docker-compose.dev.yaml` adds `dev:up` → `docker compose -f docker-compose.dev.yaml up`, `dev:down`, and so on. |

Types that run a program to list their commands (make, gradle, maven, python) are
cached in `~/.paleta/cache/types/`. The selector shows the cached list
immediately and refreshes it in the background.

You can add your own types; see [Custom project types](#custom-project-types).

## Glob locations

A location that ends in `/*` matches every directory one level below that path.
Each matching folder becomes its own location. It is named after the folder and
inherits the pattern's `type`, `commands`, `env` and filters:

```yaml
locations:
  - location: "packages/*"      # every directory in packages/
    type: "npm"
```

The `*` must be a single star at the end, directly after a `/`. `packages/*` is
valid; `apps/*/backend` and `packages/**` are not, and plt rejects them when it
loads the config. Only directories match; files are skipped.

### Excluding folders

`exclude_locations` drops folders from the expansion. Patterns are matched against
the folder's own name:

```yaml
locations:
  - location: "services/*"
    type: "npm"
    exclude_locations:
      - "legacy"            # exact folder name
      - "*-deprecated"      # or a pattern
```

A pattern that matches nothing is fine, so you can exclude a folder before it
exists.

### Per-folder overrides

`overrides` changes individual folders of a glob. Keys are matched against folder
names the same way: `frontend-next` hits one folder and `*-worker` hits several.

```yaml
locations:
  - location: "services/*"
    type: "npm"
    commands: ["foo", "bar"]
    env:
      LOG: "info"
    overrides:
      frontend-next:
        commands:
          - name: "commandx"
            command: "npm run commandx"
        env:
          PORT: "3001"
      "*-worker":
        type: ["npm", "docker"]
```

How each field merges into the folder's location:

| Field | Merge |
|---|---|
| `commands` | Added. A command whose `name` matches an inherited one replaces it in place. |
| `env` | Merged per key; the override wins. |
| `name`, `type`, `include_commands`, `exclude_commands` | Replace the inherited value. |

Because these fields are replaced rather than merged, one folder can narrow a
filter or drop a type its siblings have. To *remove* an inherited command from
one folder, exclude it by name:

```yaml
    overrides:
      legacy-svc:
        exclude_commands: ["bar"]
```

`plt lint` reports an override key that matches no folder, which is usually a
typo or a renamed directory. Excluding a folder that still has an override is not
reported, so you can use both keys together.

Prefer leaving `name:` off a glob location. Without it, each folder is named
after itself, which keeps `@project:command` references unambiguous. If a
single folder needs a different name, give it one in its override.

## Commands

`commands` accepts two forms, and you can mix them in one location.

**String form:**

```yaml
commands:
  - "npm run build"
  - "./deploy.sh"
```

**Object form (recommended):** the name is shown in the selector, filters can
match it, and other commands can reference it:

```yaml
commands:
  - name: "build"
    command: "npm run build"
  - name: "deploy"
    command: "./deploy.sh"
```

A string-form command has no name, so filters match its full command string.

**List form (chained commands):** `command` can be a list instead of an inline
`a && b` string. The items are joined with `&&`:

```yaml
commands:
  - name: "ci-and-dev"
    command:
      - pnpm install
      - pnpm dev
    # runs as: pnpm install && pnpm dev
```

A single-item list is the same as a plain string. When you save a multi-command
queue from the selector (`s` in the queue editor), plt writes it in this form.

Commands run in the location's directory.

## Command references

A command can reference another command instead of repeating it, using
`@project[type]:command`:

- **`@project`** is the target location, by its `name` (or the folder's name).
- **`[type]`** is *optional*. You only need it when the target location has more
  than one type (e.g. `[npm]` vs `[docker]`).
- **`:command`** is the name of the referenced command.

References expand when the config loads, so you can combine them with `&&`:

```yaml
locations:
  - name: web
    location: packages/web
    type: pnpm
  - name: api
    location: services/api
    type: go
    commands:
      # A reference to another project runs in that project's directory
      # (wrapped in a subshell), then continues here:
      - name: ci
        command: "@web:build && go test ./..."
        # expands to: (cd '…/packages/web' && pnpm run build) && go test ./...
```

A reference to the same project stays bare (`@web:build` inside `web` →
`pnpm run build`). An unresolvable reference does not stop the rest of the config
from loading. That includes an unknown project or command, a command on a
multi-type location written without `[type]`, and a reference cycle. The command
is flagged in the selector and reported by `plt lint`.

Glob locations expand to one location per folder, each named after its folder, so
`@web:build` targets `packages/web` specifically.

When two folders share a name (e.g. `packages/search` **and** `services/search`),
the bare `@search:build` is ambiguous. Use the end of the path instead; the
project part accepts paths too:

```
@packages/search:build
@services/search:test
```

## Environment variables

You can set environment variables for a whole **location** (every command in it)
or for a single **command**. If both set the same key, the command's value wins.
Overrides and tools follow the same rule.

```yaml
locations:
  - location: "api"
    env:
      BIN: "${HOME}/.local/bin"   # references the ambient environment
      PATH: "${BIN}:$PATH"        # references a sibling var + ambient
      PORT: "3000"
    commands:
      - name: "dev"
        command: "npm run dev"
        env:
          PORT: "3001"            # overrides the location-level PORT
```

- Values may reference the environment plt was started in (`${HOME}`, `$PATH`).
- Values may reference other variables defined at the same level, in any order.
- An undefined variable expands to an empty string.

The variables apply only while the selected command runs. They do not leak into
your shell, and when you run several commands at once each one gets its own env.

## Command filtering

`include_commands` and `exclude_commands` filter a location's commands:

```yaml
locations:
  - location: "packages/api"
    type: "npm"
    include_commands:
      - "test*"     # only commands starting with "test"
      - "build"     # and "build"
    exclude_commands:
      - "*:watch"   # but no watch commands
```

- Patterns match the command's **name**. A command without a name is matched by
  its full command string.
- Patterns are globs (`*`, `?`, …).
- `include_commands` is an allow-list (only applied if present). `exclude_commands`
  is a deny-list, applied after it.
- Filters apply to discovered **and** hand-written commands, so an allow-list
  must also cover the commands you wrote.

## Focus

`focused` lists the locations the selector shows by default. You can still
reach every other location from the selector. Each entry matches a location's
`name`, or its `location` path when it has no name (`.` for the root).
`plt focus` and `Ctrl+P` in the selector edit this list for you.

```yaml
focused:
  - web
  - "services/*"
```

`focused` matches what you wrote in the config, before globs expand, so a glob
location can only be focused as a whole. Entries that match nothing are ignored.

## Frecency

plt ranks commands by how often and how recently you ran them:

```yaml
frecency:
  enabled: true
  recency_weight: 50
  frequency_weight: 50
```

Only the ratio of the two weights matters, so `50`/`50` and `0.5`/`0.5` behave
the same. A `frecency` block in `.pltrc` overrides the one in the global
`~/.config/paleta/config.yaml`.

## Tools

Tools are programs like `lazygit` that aren't tied to a location. They appear at
the **end** of the selector and run in your **current directory**. Enable them by
name:

```yaml
tools:
  - lazygit
  - docker
```

Names resolve against a built-in registry plus definitions in your global config:

| Tool | Rows |
|---|---|
| `lazygit` | `lazygit` |
| `lazydocker` | `lazydocker` |
| `docker` | `docker: up` / `down` / `logs` / `ps` |

You define your own tools (or override a built-in tool of the same name) in
`~/.config/paleta/config.yaml`, not in `.pltrc`. There, `tools` is a map:

```yaml
tools:
  gitui:                          # single command → one row "gitui"
    command: gitui
  compose:                        # several commands → "compose: up", "compose: logs"
    env:
      COMPOSE_PROJECT_NAME: myapp # tool-level env (per-command env wins)
    commands:
      - name: up
        command: docker compose up
      - name: logs
        command: docker compose logs -f
```

Enabling a name that is neither built in nor defined globally produces a warning,
not an error: the selector banner and `plt lint` report it, and everything else
still loads.

## Custom project types

Define your own project types (or override a built-in one by name) in
`~/.paleta/parsers.yaml`:

```yaml
parsers:
  just:
    detect_files: ["justfile", "Justfile"]   # any of these marks a folder as this type
    priority: 65                            # order among types found in one folder (lower first)
    base_commands:
      list: "just --list"
    parser_command: "just --summary | tr ' ' '\n'"   # prints one command name per line
    command_template: "just {key}"          # turns each discovered name into a command
```

Instead of `parser_command`, you can set `builtin_parser: package_json_scripts`
(or `go_standard`) to read the command names from a file. Types with a
`parser_command` are resolved in the background, so a slow tool never delays the
palette. Types without a `priority` sort after all types that have one.

## Global project configuration

Instead of a `.pltrc` inside the project, you can keep the config in
`~/.config/paleta/projects/<anything>.yaml`. It uses the same format, plus
`root`:

```yaml
root: /home/user/projects/my-project
locations:
  - name: "frontend"
    location: "packages/frontend"
    type: "npm"
```

1. plt first looks for a `.pltrc` in the current directory and its parents. If it
   finds one, that file wins.
2. Otherwise, plt scans `~/.config/paleta/projects/*.yaml` for configs whose
   `root` is the current directory or one of its parents.
3. If several match, the closest (most specific) `root` wins.

## Checking your edits

```bash
plt lint          # report problems in the .pltrc
plt lint --fix    # fix what can be fixed automatically
```

`plt lint` reports:

- names that can't be referenced
- unresolved `@project:command` references
- deprecated key spellings
- `overrides` keys that match no folder
- unknown tools

`--fix` renames deprecated keys and rewrites invalid name characters to `_`. It
keeps your comments and formatting. You must fix unresolved references by hand.

**Note:** plt ignores keys it does not recognise, and `plt lint` does not report
them yet. Check spellings against the tables above. A misspelled
`exclude_location:`, for example, silently does nothing.

## Examples

### npm monorepo

```yaml
locations:
  - name: "web-app"
    location: "apps/web"
    type: "npm"
  - name: "mobile-app"
    location: "apps/mobile"
    type: "npm"
  - location: "packages/*"
    type: "npm"
```

### Mixed project types

```yaml
locations:
  - name: "frontend"
    location: "frontend"
    type: "npm"
  - name: "backend"
    location: "backend"
    type: "go"
  - name: "scripts"
    location: "scripts"
    commands:
      - name: "deploy-prod"
        command: "./deploy.sh production"
      - name: "deploy-staging"
        command: "./deploy.sh staging"
      - name: "backup"
        command: "./backup.sh"
```

### Custom commands with filtering

```yaml
locations:
  - name: "api"
    location: "services/api"
    type: "npm"
    commands:
      - name: "docker-up"
        command: "docker compose up"
      - name: "docker-down"
        command: "docker compose down"
      - name: "migrate"
        command: "make migrate"
    include_commands:
      - "test*"      # test scripts from package.json
      - "build"
      - "docker-*"   # our own docker commands
      - "migrate"
    exclude_commands:
      - "test:e2e"
```

### Mixed command forms

```yaml
locations:
  - name: "frontend"
    location: "apps/frontend"
    type: "npm"
    commands:
      - name: "dev-local"
        command: "npm run dev -- --host 0.0.0.0 --port 3000"
      - "npm run storybook"   # string form
    include_commands:
      - "build*"
      - "dev-*"
      - "npm run storybook"   # a command without a name is matched by its command string
```
