# mapping-setup

An installer that sets up a complete **Wolfenstein: Enemy Territory / ETJump
mapping environment** in one go: game engine, game data, ETJump, NetRadiant-custom,
the mapping SDK and all extra assets, fully configured.

It automates the steps in the
[ETJump mapping guide](https://etjump.readthedocs.io/en/latest/mapping/getting_started.html).
Each platform gets a single self-contained program; nothing else needs to be
installed to run it.

- [Using the installer](#using-the-installer)
- [Contributing](#contributing)

---

## Using the installer

### 1. Download

Open the **Releases** page of this repository and download the file for your
system from the latest release:

| System | File |
|---|---|
| Windows (64-bit) | `etmapping-setup-windows.exe` |
| Linux (64-bit) | `etmapping-setup-linux` |
| macOS, Apple Silicon (M1 and newer) | `etmapping-setup-macos-arm64` |
| macOS, Intel | `etmapping-setup-macos-intel` |

Each release also lists a `SHA256SUMS` file to check the download if you want to.

### 2. Run it

**Windows:** double-click the `.exe`. If Windows SmartScreen warns about an
unknown publisher, click *More info → Run anyway* (the installer is not code-signed).

**Linux:** open a terminal in the download folder, then:

```bash
chmod +x etmapping-setup-linux
./etmapping-setup-linux
```

Run it as your normal user, not with `sudo`.

**macOS:** open Terminal in the download folder, then:

```bash
chmod +x etmapping-setup-macos-arm64
./etmapping-setup-macos-arm64
```

If macOS says the developer cannot be verified, open *System Settings →
Privacy & Security* and click *Open Anyway*, then run it again.

### 3. Answer the questions

All questions come first; nothing is downloaded until you confirm.

| Question | What to choose |
|---|---|
| **Install folder** | Where the mapping setup goes. Press Enter for the default (`ETMapping` in your home folder) or type another path. Tab completes folder names. The folder must be new or empty, and the path may only contain English letters (NetRadiant and the map compiler cannot handle others). |
| **Game engine and architecture** | ET: Legacy, ETe or ET 2.60b, 64- or 32-bit. If unsure, ET: Legacy 64-bit works everywhere. |
| **NetRadiant-custom** | *Install it*, or *I already have a level editor* and give its folder (the one containing `gamepacks`). |
| **Extras** | Optional tools for packing maps into pk3 files: Pack3r (command line) and QtPack3r (graphical). Space toggles, Enter continues. |

Options that do not work on your system are shown greyed out with the reason:

| | Windows | Linux | macOS |
|---|:-:|:-:|:-:|
| ET: Legacy 64-bit | ✔ | ✔ | ✔ |
| ET: Legacy 32-bit | ✔ | ✔ | |
| ETe 64-bit / 32-bit | ✔ | ✔ | |
| ET 2.60b (32-bit) | ✔ | | |
| Install NetRadiant-custom | ✔ | ✔ | |
| Use an existing editor | ✔ | ✔ | ✔ |
| Pack3r, QtPack3r | ✔ | ✔ | |

You then get a summary with every component, its version and download size
(about 300–400 MB in total). Confirm to start; the rest runs on its own.

Press Ctrl+C or Esc at any question to cancel without installing anything.

### What gets installed

Always the latest versions, looked up when the installer runs:

| Component | Version used |
|---|---|
| ET: Legacy / ETe / ET 2.60b | newest ET: Legacy release · ETe's latest build · the original 2.60b |
| Game data (`pak0-2.pk3`, plus `mp_bin.pk3` for 32-bit) | the original files, checksum-verified |
| ETJump | newest numbered release (development builds are skipped) |
| NetRadiant-custom | newest release |
| ET mapping SDK, ETJump mapping assets, ET extra assets | current versions |
| Pack3r, QtPack3r (if chosen) | Pack3r's latest build · newest QtPack3r release |

And then configures everything the guide asks for:

- `lightblock` and `blitz_sd_mm` added to `etmain/scripts/shaderlist.txt`
- the missing editor image fixed in `chateau.shader`
- ETJump's `et_entities.ent` copied into the editor (the original is kept as `et_entities.ent.orig`)
- NetRadiant-custom set up to open straight into ET with the right engine path
- for ET 2.60b and ETe on Windows: `cgame_mp_x86.dll` and `ui_mp_x86.dll` extracted next to
  their pk3s, because 2.60b only extracts them itself when joining a server

The result looks like this (Windows, ET: Legacy):

```
ETMapping/
├── etl.exe                     game engine
├── etmain/                     game data, SDK and mapping assets
├── etjump/                     ETJump
├── netradiant-custom/          the editor (radiant.exe; on Linux: AppRun)
├── tools/pack3r, tools/qtpack3r
├── etmapping-setup.json        what was installed (used for updates)
└── etmapping-setup.log         details of every run
```

### After installing

- **Map:** start NetRadiant-custom (the installer offers to open it). ETJump entities
  are in the right-click menu of the 2D view, grouped with the stock ones
  (e.g. *target → target_starttimer*).
- **Test your map:** start the game with `+set fs_game etjump`, e.g.
  `etl.exe +set fs_game etjump`, then `/devmap yourmap` in the console.
- **Linux:** the final screen lists any system packages still needed (for
  32-bit engines or QtPack3r) with the exact install command for your distribution.

Two ETJump entities (`alarm_box`, `props_chair_chat`) use models that only exist
in RTCW and cannot be included.

### Updating, resuming, uninstalling

- **Update:** run the installer again and pick the same folder. It offers to update
  everything to the latest versions with your previous answers. Your maps and
  `.pk3dir` folders are never touched, and NetRadiant's settings are kept.
- **Resume:** if an install was interrupted, run it again with the same folder;
  finished downloads are reused.
- **Uninstall:** delete the install folder. Nothing is written anywhere else.

### Troubleshooting

| Problem | Fix |
|---|---|
| "cannot reach …" at start | Check your internet connection or firewall; the installer needs github.com, etlegacy.com, mirror.etlegacy.com and aciz.etjump.com. |
| An install step fails | Read `etmapping-setup.log` in the install folder, then run the installer again to resume. |
| GitHub rate limit when running it many times | The installer falls back automatically; setting a `GITHUB_TOKEN` environment variable avoids the limit entirely. |
| Prompts look broken | Run it in a regular terminal (Windows Terminal, PowerShell, a Linux/macOS terminal), not in an IDE's output panel or the MSYS2 window. |

---

## Contributing

### Requirements

- [Go](https://go.dev/dl/) 1.27 or newer
- `make` in a POSIX shell (MSYS2 or Git Bash on Windows; any shell on Linux/macOS)

### Building

| Command | Result |
|---|---|
| `make` | all four programs in `dist/` |
| `make windows` / `make linux` / `make macos` | one platform (macOS builds both Apple Silicon and Intel) |
| `make run` | run from source on this machine |
| `make vet` | static checks; run before sending changes |
| `make VERSION=1.2.0` | stamp a version into the programs (shown in the banner and log) |
| `make clean` | delete `dist/` |

Every platform builds from any platform (Go cross-compiles), so a Windows
machine produces the Linux and macOS programs too. Format code with `gofmt`.

### How it works

1. **Checks:** supported platform, not running as root, servers reachable.
2. **Questions:** all answers are collected first (`cmd/etmapping-setup/main.go`).
3. **Resolve:** each component looks up its latest version and download URLs,
   producing a *plan* (`install.Resolve`).
4. **Summary and confirmation.**
5. **Run:** every component is downloaded into a cache inside the install folder
   and installed, then the file edits are applied (`install.Run`). A manifest
   marks the install as in progress until it completes, which is how resuming
   and updating work.

### Code layout

| Package | Purpose |
|---|---|
| `cmd/etmapping-setup` | entry point: checks, questions, summary, install, final report |
| `internal/platform` | OS detection and the table of which option is available where |
| `internal/ui` | questions (Bubble Tea), Tab completion, console output and log |
| `internal/fetch` | downloads (cache, resume, retries, SHA-256) and GitHub lookups |
| `internal/extract` | unpacking zip, tar.gz, 7z and AppImage files safely (nothing can be written outside the target folder) |
| `internal/sources` | one file per thing being installed: where it comes from, how its latest version is found, how it is put in place |
| `internal/install` | the plan, running it, file edits, NetRadiant settings, folder checks, Linux hints |

### Common changes

**A download source moved or renamed its files.** Edit that component's file in
`internal/sources/`. Each one returns a `Component` with its version, the files
to download and an `Install` function.

**Adding a component.** Write a function in `internal/sources/` that returns a
`Component`, then add it to the list in `install.Resolve` (`internal/install/install.go`).
If users should be able to choose it, add an `Option` in `internal/platform/options.go`
and handle it in `ask` in `main.go`.

**Changing which options work on which platform.** Edit the `Platforms` list of the
option in `internal/platform/options.go`. The questions, greyed-out entries and
summary all follow from it.

**A fixed file changed.** The original game files and ET 2.60b are pinned by
SHA-256 in `internal/sources/gamedata.go` and `internal/sources/engine.go`.

### Things worth knowing

- **GitHub lookups** use the API first and fall back to github.com pages when the
  anonymous limit (60 requests/hour) is hit, so both paths need to keep working.
  Branch and tag lists come from git's own HTTP endpoint, which has no limit.
- **ET: Legacy** publishes downloads only on etlegacy.com; the installer reads the
  download page and picks the file by name. If their site changes, look at
  `etLegacy` in `internal/sources/engine.go`.
- **NetRadiant-custom on Linux** is an AppImage. It is unpacked during install so
  its game pack and settings folder can be written to (`internal/extract/appimage.go`).
- **NetRadiant settings** are written in its *portable* format: a `settings/` folder
  next to the program, which keeps everything inside the install folder.

### Testing changes

There are no automated tests yet. Before sending a change:

1. `gofmt -l .` (should print nothing), `make vet` and `make`.
2. Run the installer into an empty test folder with the options your change affects,
   and check the result and `etmapping-setup.log`.
3. Run it again on the same folder to check that updating still works.
4. If you changed something platform-specific, test on that platform or ask
   someone who uses it.

Every push and pull request is checked by GitHub Actions
(`.github/workflows/build.yml`): formatting, `go vet` for Windows, Linux and macOS,
and a build of all four programs. The built installers are attached to each run
(the *installers* artifact on the run's page), so testers can try a pull request
without building it themselves.

### Releasing

Push a version tag:

```bash
git tag v1.0.0
git push origin v1.0.0
```

GitHub Actions builds the installers with that version stamped in and publishes
a release with them and a `SHA256SUMS` file. Tags with a dash, like `v1.1.0-rc1`,
are published as pre-releases. Release notes are generated from the commits and
pull requests since the previous tag and can be edited on GitHub afterwards.

---

## License

BSD 3-Clause, see [LICENSE](LICENSE).
