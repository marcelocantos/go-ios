# Entropy audit — go-ios

Date: 2026-08-22
Mode: full (entropy + hygiene)

## Executive summary

- **Snapshot:** `/Users/marcelo/work/github.com/marcelocantos/go-ios`
  - Branch: `spyder-patches` (tracks `origin/spyder-patches`)
  - HEAD: `d78573d186a386b488d839144dffdf9539bf58d1` (`tunnel: self-heal a userspace tunnel whose device lifeline dies`)
  - Initial dirty state: clean (`git status --porcelain=v1 -b` showed only `## spyder-patches...origin/spyder-patches`)
  - Remote default branch: `main` (this fork of `github.com/danielpaulus/go-ios`; module path is still `github.com/danielpaulus/go-ios`)
  - This branch is **one commit** ahead of `origin/main` (tunnel self-heal).
- **Scope:** Go CLI (`main.go`), library (`ios/`), REST sibling (`restapi/`), NCM driver (`ncm/` + `cmd/cdc-ncm`), npm wrapper (`npm_publish/`), CI/release, Makefile. Languages judged: Go (primary), JavaScript (npm publish), POSIX shell (`configure`, `.githooks/pre-commit`, Makefile recipes). Objective-C (`ios/opack/main.m`) and Swift (`ios/nskeyedarchiver/swiftscripts/`) are fixture generators, not product.
- **Exclusions (named, not silent):** protocol fixtures under `ios/dtx_codec/fixtures/`, `ios/nskeyedarchiver/fixtures/`, `ios/test-fixture/`, `ios/xpc/*.bin`, `ios/testmanagerd/testdata/`; opaque binaries `testdata/{app-signer-linux,app-signer-mac,wda.ipa}` (~15 MB, not Git LFS); gitignored generated `restapi/docs`.
- **Headline mechanism:** A hub-and-spoke iOS protocol library with a 3 014-line CLI dispatcher, an iOS-17 transport stack that every service re-selects ad hoc, and two sibling deployables (`restapi`, `ncm`) that are not on the same compile/test/release path as the library they ride.
- **Highest-consequence findings:** REST API does not compile against the workspace library; iOS 17+ connection choice is copied per service; CLI change amplification in `main.go`; XCUITest three-way protocol forks; “fast” CI tests are not hermetic.
- **Unverified residue:** live iOS-device journeys (install, tunnel, XCTest, WDA) were not run; `govulncheck` is not installed; upstream CVE status of gin 1.8.1 / quic-go / golang.org/x/crypto 0.24.0 was not re-scored; Windows/`GOOS=linux` ncm USB path not executed here.

## Dimension vector

| Dimension | State | Evidence summary | Change from baseline |
|---|---|---|---|
| Architecture topology | concern | Hub `ios` with acyclic leaves is coherent; iOS 17 transport, REST, and NCM sit beside it without a shared adapter or build graph | n/a (first audit) |
| Redundancy / sources of truth | concern | Four connect primitives, three XCTest runners, two location stacks, two file APIs, three version strings, two npm installers | n/a |
| Change amplification | concern | `main.go` 117 commits / 3 014 lines; a protocol bump fans out across CLI + per-service connect + REST | n/a |
| Local code quality | concern | Production panics (`DeviceConnectionRWC`, debugproxy, pair decode); `WaitUntilAgentReady` busy-loop; golangci only `forbidigo` | n/a |
| Correctness / verification | concern | REST sibling does not compile; imagemounter “fast” tests hit the network and panic; Windows tests commented out; most service packages have no tests | n/a |
| Security / dependencies | concern | REST binds `:8080` with no auth; TSS client `InsecureSkipVerify` to `gs.apple.com`; no vuln scan; opaque signer binaries | n/a |
| Build / release / operations | concern | Version via `sed` of `main.go`; NCM needs `go.work`; REST swagger generated at deploy; npm layout disagrees with `index.js` | n/a |
| Documentation / governance | concern | No `AGENTS.md` / `hygiene.yaml` / CODEOWNERS; REST README still a TODO list; CONTRIBUTING targets upstream `main` | n/a |

Do not collapse this vector to a score.

## Scope and exclusions

**In scope:** shipped CLI `ios`, Go module `github.com/danielpaulus/go-ios`, `restapi`, `ncm`/`cmd/cdc-ncm`, `npm_publish`, `.github/workflows`, Makefile, `configure`.

**Excluded from production-quality conclusions (still inventoried):**

- `ios/dtx_codec/fixtures/` (~2.1 MB captured DTX)
- `ios/nskeyedarchiver/fixtures/` (plist/bin archives)
- `ios/test-fixture/`, `ios/xpc/*.bin`, `ios/testmanagerd/testdata/*.xctestrun`
- `testdata/app-signer-{linux,mac}`, `testdata/wda.ipa` (unsigned 15 MB blobs; see ENT-018)
- `restapi/docs` (gitignored swagger output)
- `ios/opack/main.m`, `ios/nskeyedarchiver/swiftscripts/createSampleArchives.swift` (generators)

**Not present:** `AGENTS.md`, `CLAUDE.md`, `hygiene.yaml`, `docs/audits/` (created by this report), `.gitattributes`, CODEOWNERS, Dependabot config (stale branches exist).

## Commands run

| Command | Version / notes | Exit | Shipped vs auxiliary | Limitations |
|---|---|---|---|---|
| `git rev-parse HEAD`; `git status --porcelain=v1 -b` | git | 0 | provenance | Snapshot before any write |
| `go version` | `go1.26.4 darwin/arm64` (module toolchain is `go1.22.5`) | 0 | auxiliary | Local toolchain newer than declared |
| `go list ./...` (root) | lists 38 packages | 0 | auxiliary | Workspace `go.work` active |
| `go list ./...` in `restapi/` | fails on missing `restapi/docs` | 1 | shipped REST | See ENT-001 |
| `go build ./...` (root) | empty output | 0 | shipped CLI/library | Does not include restapi module compile of `main` |
| `go list -f imports` internal graph | python reduction | 0 | auxiliary | No cycles |
| `go test -count=1 -tags=fast ./...` | CI command | killed at 180 s | shipped CI path | Completed `ios`, `afc`, `dtx_codec`, `house_arrest` (`[no tests to run]`); hung later in imagemounter |
| `go test -count=1 -timeout 60s -tags=fast ./ios/imagemounter ./ios/instruments ./ios/nskeyedarchiver ./ios/opack ./ios/ostrace ./ios/springboard ./ios/testmanagerd ./ios/tunnel ./ios/xpc ./ios/zipconduit ./ios/fileservice` | local | 1 | mixed | `imagemounter` FAIL (network + WaitGroup panic + 60 s timeout on `TestWorksWithoutProxy`); others `ok` or no tests |
| `cd ncm && go test -count=1 -timeout 30s ./...` | | 0 | ncm shipped tests | Linux USB path not exercised on darwin |
| `cd restapi && go test ./api` | | 1 | shipped REST | **build failed** (type mismatch) |
| `cd restapi && go build .` | | 1 | shipped REST | missing `restapi/docs` |
| `go build ./cmd/cdc-ncm` | GOWORK on | 0 | ncm | Requires workspace |
| `GOWORK=off go build ./cmd/cdc-ncm` | | 1 | ncm | `package go-ios-cdcncm is not in std` |
| `golangci-lint run ./...` | 2.12.2 (CI pins action v2.6) | 0 | CI lint job | Config enables only `forbidigo` |
| `gofmt -l .` | | 0, empty | CI “Verify code formatting” | Matches CI |
| `command -v govulncheck` | not installed | — | — | Residue |
| `ls hygiene.yaml` | absent | 1 | hygiene | Posture undeclared; validator not run |

## Observed architecture

### Deployable units

```
ios CLI (main.go) ──imports──► ios/* service packages
                                │
                                ▼
                         package ios
                    (usbmux, lockdown, pair,
                     ConnectTo*, RSD, DeviceEntry)
                                │
                    ┌───────────┼───────────┐
                    ▼           ▼           ▼
                 dtx_codec   xpc/http    nskeyedarchiver
                 instruments  fileservice  opack
                 testmanagerd tunnel

restapi (gin :8080) ──replace ../──► same ios library
ncm (go-ios-cdcncm) ──independent──► USB CDC-NCM TAP (Linux)
cmd/cdc-ncm         ──go.work only──► ncm
npm_publish         ──ships prebuilt ios binary──► npm
```

No import cycles among `github.com/danielpaulus/go-ios...` packages. Fan-in: `ios` has 30 importers. Fan-out: root `main` imports 25 service packages.

### Declared vs observed

| Rule | Status |
|---|---|
| Static Go binaries, JSON CLI, library-as-module (README) | Observed, enforced by `go build` / `go.mod` |
| iOS 17+ needs `ios tunnel start` then RSD (README, `ios/tunnel/doc.go`) | Declared well; **not** enforced as a single transport — each service chooses usbmux / shim / tunnel / XPC (ENT-002) |
| REST is experimental (`restapi/README.md`) | Contradicted by `.github/workflows/deploy.yml` (self-hosted ngrok “staging”) |
| Swagger `BasicAuth` (`restapi/main.go:25`) | Contradicted: no auth middleware |
| golangci-lint in CI and pre-commit | Observed, but `.golangci.yml` only forbids `print`/`println` |
| `tags=fast` = hermetic unit tests | Contradicted by `imagemounter` tests (ENT-005) |
| `HttpApiPort` comment “60106” | Contradicted by constant `60105` (`ios/connect.go:327-340`) |

### Inferred (not documented as architecture)

- Pre-iOS 17: usbmuxd → lockdown `StartService` → optional SSL from pair record.
- iOS 17+: userspace or kernel TUN + RSD port map + HTTP/2+XPC or shim checkin.
- Pairing certs use SHA-1 RSA because lockdown expects it (`ios/crypto_utils.go:53`) — protocol constraint, not a local crypto fashion.
- Device TLS uses `InsecureSkipVerify` with pair-record client certs (`ios/deviceconnection.go:272-275`) — typical for this domain; TSS to Apple is a different threat (ENT-009).

### Cross-cutting concerns

- Logging: logrus in 79 files, `log/slog` in 7 (tunnel/ncm/discover).
- Errors: mixed `fmt.Errorf` wrapping and `github.com/pkg/errors`.
- CLI parsing: one giant docopt usage string in `main.go`.
- Tunnel agent HTTP on `127.0.0.1:60105` (env `GO_IOS_AGENT_HOST`/`PORT`).

## Findings

### ENT-001: REST sibling does not compile against the workspace library

- **Priority:** P1
- **Dimensions:** Correctness / verification; Architecture topology; Build / release / operations
- **Status:** observed fact
- **Evidence:**
  - `restapi/go.mod:8` requires `github.com/danielpaulus/go-ios v1.0.91` (current tags reach `v1.0.213`) with `replace => ../` at `restapi/go.mod:62`.
  - `ios/installationproxy/installationproxy.go:179-212` — `AppInfo` is `map[string]any`; `CFBundleIdentifier()` / `CFBundleExecutable()` are methods.
  - `restapi/api/app_endpoints.go:107-109` still treats those as fields: `app.CFBundleIdentifier == bundleID` → compiler: `mismatched types func() string and string`.
  - `restapi/main.go:7` imports `_ "github.com/danielpaulus/go-ios/restapi/docs"`; `restapi/.gitignore` ignores `**/docs`.
  - Commands: `cd restapi && go test ./api` exit 1; `cd restapi && go build .` exit 1 (`no required module provides package .../restapi/docs`).
- **Mechanism:** REST is a second product on a `replace` to HEAD, but its types and generated swagger were not kept in the same commit as the library. Anyone building REST from this tree fails. Anyone building REST *without* the replace gets a 122-minor-version-old library.
- **Blast radius:** `restapi` binary, `.github/workflows/deploy.yml` (`swag init` then `go build` in `restapi/`), any consumer of the experimental API (install/launch/kill/WDA).
- **Counterevidence checked:** `deploy.yml` runs `swag init` so docs can appear on that one self-hosted job; it does not fix the `AppInfo` method mismatch. Root `go test ./...` never enters the restapi module. `go.work` `use ./restapi` does not make `go test ./...` from root test it.
- **Smallest coherent remediation:** Make `AppInfo` accessors compile in `restapi/api`; check in or generate swagger in CI for restapi; bump/replace the module require to the workspace module in one place; add `go test`/`go build` of `./restapi/...` to the unit-test workflow.
- **Verification:** `go test ./api` and `go build .` from `restapi/` must pass on CI without a prior manual `swag init` on the developer laptop.
- **Ratchet candidate:** CI job `test.yml` step `go test ./...` in `restapi/` and `ncm/` (workspace modules), blocking.

### ENT-002: iOS 17 transport is four primitives re-selected in every service

- **Priority:** P1
- **Dimensions:** Architecture topology; Change amplification; Redundancy / sources of truth
- **Status:** observed fact
- **Evidence:**
  - Four entry points in `ios/connect.go:90-162`: `ConnectToService` (usbmux+lockdown), `ConnectToShimService` (RSD+checkin), `ConnectToXpcServiceTunnelIface`, `ConnectToServiceTunnelIface`.
  - DTX split: `dtx.NewUsbmuxdConnection` vs `dtx.NewTunnelConnection` (`ios/dtx_codec/connection.go:154-172`).
  - Encapsulated well in some packages: `syslog.New` (`ios/syslog/syslog.go:27-31`), `zipconduit.New` (`ios/zipconduit/zipconduit_installer.go:63-67`), `ostrace.New` (`ios/ostrace/ostrace.go:236-240`), `instruments.connectInstruments` (`ios/instruments/helper.go:39-52`).
  - Not encapsulated: `afc.New` only `ConnectToService` (`ios/afc/client.go:35`); `installationproxy.New` only usbmux (`ios/installationproxy/installationproxy.go:49-50`); `accessibility.New` only `NewUsbmuxdConnection` (`ios/accessibility/accessibility.go:15-27`); `simlocation.New` only usbmux (`ios/simlocation/simlocation.go:31`); CLI `file` uses `fileservice` (iOS 17 XPC, `main.go:1358-1376`) while `fsync` uses AFC (`main.go:1498-1502`).
- **Mechanism:** Adding or fixing an iOS 17 service means remembering which of four dialers, which service name (`*.shim.remote`, `dtservicehub`, `coredevice.*`), and whether SSL handshake-only applies (`serviceConfigurations` in `ios/connect.go:59-64`). A bug in one client’s dial does not fail others; a platform change must be repeated.
- **Blast radius:** every `ios/*` service, CLI commands that call them, REST endpoints that call them, XCTest (ENT-004).
- **Counterevidence checked:** `ios/tunnel/doc.go` documents the two-era model; RSD shim fallback in `RsdPortProviderJson.GetPort` (`ios/rsd.go:43-50`) is a real shared helper. Encapsulation in syslog/zipconduit/ostrace/instruments shows the intended pattern — it was not adopted library-wide.
- **Smallest coherent remediation:** One `Connect(device, ServiceSpec)` (or similar struct, not functional options) that picks usbmux/shim/xpc from the spec and `device.SupportsRsd()`. Migrate leftover usbmux-only clients behind it; keep AFC vs fileservice as domain difference, not dial difference.
- **Verification:** architecture test: service `New` functions must not call `ConnectToService`/`NewUsbmuxdConnection` without a documented RSD branch, except an allowlist.
- **Ratchet candidate:** `go test` + grep/ast check (or golangci forbidigo patterns) for raw `ConnectToService(` outside `ios/connect.go` and the allowlist.

### ENT-003: CLI is a 3 014-line god object and the highest-churn file

- **Priority:** P1
- **Dimensions:** Change amplification; Local code quality; Documentation / governance
- **Status:** observed fact
- **Evidence:**
  - `main.go` is 3 014 lines (largest Go file by 4×; next is `objectivec_classes.go` at 782).
  - Git name-count: `main.go` 117 commits, then `README.md` 36, `xcuitestrunner.go` 35.
  - Usage string + sequential `arguments.Bool("…")` dispatch from `Main` (`main.go:73` onward); helpers such as `processList` live in the same file (`main.go:2563-2567`).
  - `processList` does `defer service.Close()` before the nil check; `NewDeviceInfoService` returns `nil, err` (`ios/instruments/instruments_deviceinfo.go:124-128`) — nil dereference on connect failure.
- **Mechanism:** Every new `ios <cmd>` edits the usage blob, the dispatch chain, and a helper. REST does not share that dispatch, so the same feature is re-wired in `restapi/api/*_endpoints.go`. Review and regression cost concentrate here.
- **Blast radius:** all CLI users; any command addition/rename; JSON vs `--nojson` formatting.
- **Counterevidence checked:** Exporting `Main` for `main_test.go` is useful. docopt keeps a single usage contract. The library packages *are* split; only the CLI glue is monolithic. That is still the file that changes most.
- **Smallest coherent remediation:** Keep docopt usage in one place; move per-command functions into `cmd/ios/` or `cli/` files by domain (device, apps, files, tunnel, test). Fix the `processList` nil-close order immediately.
- **Verification:** `wc -l` on `package main` files under an agreed cap; `go test` covering `processList` error path.
- **Ratchet candidate:** file-size or `package main` line budget in hygiene once a split lands; test that `NewDeviceInfoService` failure does not panic.

### ENT-004: XCUITest is three copied protocol implementations

- **Priority:** P1
- **Dimensions:** Redundancy / sources of truth; Change amplification; Correctness / verification
- **Status:** observed fact
- **Evidence:**
  - Dispatcher: `RunTestWithConfig` (`ios/testmanagerd/xcuitestrunner.go:284-304`) branches `< iOS14` → `runXCUIWithBundleIdsXcode11Ctx`, `< iOS17` → `runXUITestWithBundleIdsXcode12Ctx`, else `runXUITestWithBundleIdsXcode15Ctx`.
  - `ios/testmanagerd/xcuitestrunner_11.go:14-60` and `ios/testmanagerd/xcuitestrunner_12.go:16-59` duplicate: two DTX connections, `newDtxProxyWithConfig` twice, process control, authorize, start plan. Xcode 15 path (`ios/testmanagerd/xcuitestrunner.go:307-410`) repeats the shape with `NewTunnelConnection` + `appservice`.
  - Protocol version hardcoded `uint64(25)` with `// TODO: fixme` (`ios/testmanagerd/xcuitestrunner_11.go:42`).
  - Integration test is `//go:build !fast` (`ios/testmanagerd/xcuitestrunner_test.go:1`); CI never runs it.
- **Mechanism:** A handshake/capability fix must be applied three times or it is iOS-version-specific residue. Drift is already visible (Xcode 11 still uses `initiateSessionWithIdentifier` vs 12/15 capabilities).
- **Blast radius:** `ios runtest` / `runwda` / `runxctest`, REST WDA sessions (`restapi/api/wda.go`), Sauce/Headspin-style users named in README.
- **Counterevidence checked:** Apple’s testmanagerd *did* change across Xcode 11/12/15; some duplication is compatibility residue. Shared `setupXcuiTest` / `testlistener` already exist — the session orchestration was not pulled into them. Unit tests cover xctestrun parsing and listener, not the three runners’ handshake.
- **Smallest coherent remediation:** Extract “two DTX channels + authorize + start plan + wait listener” with version-specific strategy hooks (service name, capability payload, how to start the runner). Keep the three service names as data.
- **Verification:** table-driven fake DTX covering 11/12/15 handshake sequences; CI `!fast` or a recorded-fixture runner for the handshake.
- **Ratchet candidate:** test file that fails if a new `runX*WithBundleIds*` appears without sharing the extracted orchestrator.

### ENT-005: `tags=fast` is not a hermetic oracle

- **Priority:** P1
- **Dimensions:** Correctness / verification
- **Status:** observed fact
- **Evidence:**
  - CI: `.github/workflows/test.yml:63-64` `go test -v -tags=fast ./...` on Linux only (Windows equivalent is commented at lines 41-42).
  - `ios/imagemounter/imagedownloader_test.go:28-74` `TestUsesProxy` starts `http.ListenAndServe(":60001", proxy)`, calls `Download17Plus` (live `https://deviceboxhq.com/ddi-15F31d.zip` log line), `wg.Done()` on **every** CONNECT. Second CONNECT (`gs.apple.com`) panics `sync: negative WaitGroup counter` at `ios/imagemounter/imagedownloader_test.go:36`.
  - `TestWorksWithoutProxy` (`ios/imagemounter/imagedownloader_test.go:76-104`) downloads from the network, then if a device is attached calls `MountImage` → TSS (`ios/imagemounter/tss.go:84` `https://gs.apple.com/TSS/controller?action=2`). Timed out at 60 s in this environment with a device present.
  - Several real integration tests correctly use `//go:build !fast` (syslog, appservice, deviceinfo, listdevices, processcontrol, xcuitestrunner, fileservice, springboard). imagemounter tests do **not**.
- **Mechanism:** “fast” is supposed to be the PR gate. A live download, a global HTTP proxy mutation (`ios.UseHttpProxy`), a process-wide `:60001` server, and a device-dependent mount mean CI is network-flaky and can panic. Local `go test` is not a stable oracle.
- **Blast radius:** every PR; imagemounter correctness; developers with a phone plugged in.
- **Counterevidence checked:** `TestVersionMatching` is hermetic and useful. `house_arrest` with `-tags=fast` reports `[no tests to run]` — the tag works when applied. imagemounter is the breach.
- **Smallest coherent remediation:** `//go:build !fast` on tests that touch network/device; httptest the downloader; never `UseHttpProxy` without restoring `http.DefaultTransport`; `wg.Add` per expected CONNECT or use a channel.
- **Verification:** `go test -tags=fast ./...` with `HTTP_PROXY` unset and network blocked (or in CI `no-network`) must pass.
- **Ratchet candidate:** CI step that runs fast tests with network off, or a test helper that fails if `Download17Plus` is invoked under `fast`.

### ENT-006: REST API is unauthenticated on `:8080` and deployable via ngrok

- **Priority:** P2
- **Dimensions:** Security / dependencies; Build / release / operations
- **Status:** observed fact
- **Evidence:**
  - `restapi/api/server.go:13-25` `gin.Default()`, `router.Run(":8080")`, swagger at `/swagger/*any`, no auth middleware.
  - `restapi/main.go:25` `// @securityDefinitions.basic  BasicAuth` — documentation only.
  - Routes include activate, pair, install, launch, kill, WDA (`restapi/api/routes.go:18-54`).
  - `.github/workflows/deploy.yml:1-49` `workflow_dispatch` on **self-hosted**, reads ngrok public URL, copies binary to `/home/ganjalf/`, `systemctl --user start goios`.
- **Mechanism:** Anyone who can reach :8080 can pair/install/activate. The deploy workflow publishes that surface to a public ngrok URL without an auth gate. Swagger advertises BasicAuth that does not exist.
- **Blast radius:** devices attached to the host running restapi; the self-hosted runner named in deploy.yml.
- **Counterevidence checked:** README calls REST experimental. Tunnel info API binds `127.0.0.1` (`ios/tunnel/tunnel_api.go:165`) — that one is localhost-only. REST is not. DeviceMiddleware only checks UDID presence (`restapi/api/middleware.go:18-55`).
- **Smallest coherent remediation:** Bind localhost by default; require a token/basic auth matching the swagger comment; do not expose ngrok without it. Treat `/home/ganjalf/` as personal residue — move or delete the workflow if this fork does not run that host.
- **Verification:** integration test: unauthenticated POST `/api/v1/device/:udid/erase` (or pair) returns 401 when auth is on; listen address is 127.0.0.1 unless opted out.
- **Ratchet candidate:** test or lint that `router.Run` is not `:8080` without auth middleware; delete or gate `deploy.yml`.

### ENT-007: Three competing version authorities, one of them `sed` on source

- **Priority:** P2
- **Dimensions:** Redundancy / sources of truth; Build / release / operations
- **Status:** observed fact
- **Evidence:**
  - CLI: `const version = "local-build"` (`main.go:70`).
  - Release: `.github/workflows/release.yml:71,127` `gsed`/`sed` rewrite that const; Windows job does the same at lines 28-29. Tag created by `zendesk/action-create-release` on **push to `main`**.
  - REST: `GetVersion` reads `version.txt` (`restapi/api/util.go:20-27`).
  - npm: `npm_publish/package.json` `"version": "local-build"` rewritten at `.github/workflows/release.yml:160`.
- **Mechanism:** A release mutates source in the CI workspace (not via ldflags). Local builds always report `local-build`. REST can disagree with the CLI on the same host. Push-to-main on this **fork** would also `npm publish` as `go-ios` if `NODE_AUTH_TOKEN` is set — supply-chain risk for the upstream package name.
- **Blast radius:** support, npm consumers, Go module pseudo-versions vs CLI `--version`.
- **Counterevidence checked:** ldflags are already used for `-s -w` on the same job; they are not used for version. Tags `v1.0.x` exist.
- **Smallest coherent remediation:** `-ldflags "-X main.version={{tag}}"`; one version file or `runtime/debug.ReadBuildInfo`; disable npm publish on this fork or change the package name.
- **Verification:** `ios --version` after `go build` without sed equals `git describe`; REST `/version` matches.
- **Ratchet candidate:** CI asserts the built binary’s version equals the tag; forbid `sed` of `main.go` in workflows.

### ENT-008: NCM is a workspace-only module with a non-importable path

- **Priority:** P2
- **Dimensions:** Architecture topology; Build / release / operations
- **Status:** observed fact
- **Evidence:**
  - `ncm/go.mod:1` `module go-ios-cdcncm` (not `github.com/danielpaulus/go-ios/...`).
  - `cmd/cdc-ncm/main.go:5` `import ncm "go-ios-cdcncm"`.
  - `go.work` uses `.`, `./ncm`, `./restapi`.
  - `GOWORK=off go build ./cmd/cdc-ncm` → `package go-ios-cdcncm is not in std`.
  - Makefile `build` runs `go work use .` then `go work use ./ncm` (mutates workspace selection).
  - `ncm/ncm.go:156` `panic("not implemented :-)")` on chained NDP (`NextNpdIndex != 0`).
- **Mechanism:** The Linux USB driver cannot be built from the root module alone. CI `make build` happens to enable the workspace; a plain `go build ./...` from a consumer GOPATH/module cache cannot see `cmd/cdc-ncm`’s import. ncm is also untested in the root CI job.
- **Blast radius:** Linux CDC-NCM users; anyone vendoring this repo without `go.work`.
- **Counterevidence checked:** `cd ncm && go test ./...` passes (darwin). Independence from the iOS library is a *healthy* boundary (ncm does not import `ios`). The break is the module path + cmd placement.
- **Smallest coherent remediation:** Either `module github.com/danielpaulus/go-ios/ncm` and import that from `cmd/cdc-ncm`, or move `cmd/cdc-ncm` under `ncm/`. Stop toggling `go work use` in Make.
- **Verification:** `GOWORK=off go build ./cmd/cdc-ncm` succeeds; CI builds ncm on linux.
- **Ratchet candidate:** CI `GOWORK=off go build` for all `cmd/*`.

### ENT-009: Developer-image TSS talks to Apple with `InsecureSkipVerify`

- **Priority:** P2
- **Dimensions:** Security / dependencies
- **Status:** observed fact
- **Evidence:** `ios/imagemounter/tss.go:75-84` HTTP client to `https://gs.apple.com/TSS/controller?action=2` with `TLSClientConfig: &tls.Config{InsecureSkipVerify: true}`. Used by personalized DDI mount (`ios/imagemounter/personalized_image_mounter.go`, stack in ENT-005).
- **Mechanism:** A MITM on the TSS response can feed a bogus image signature. This is a public Internet endpoint, unlike device-local lockdown TLS (where skip-verify plus pair-record client certs is the usual usbmux pattern — see Healthy structure).
- **Blast radius:** `ios image auto` / `image mount` on iOS 17+; any host using a proxy (`UseHttpProxy`).
- **Counterevidence checked:** Device TLS skip-verify is commented as trusting the phone (`ios/deviceconnection.go:272-275`) and QUIC tunnel similarly (`ios/tunnel/tunnel.go:237-238`) — different trust domain. TSS has no pin and no comment why Apple’s public CA is skipped.
- **Smallest coherent remediation:** Use the system/default TLS pool for `gs.apple.com`; keep skip-verify only on the device/QUIC sockets that present pairing certs.
- **Verification:** unit test that the TSS client’s `tls.Config.InsecureSkipVerify` is false.
- **Ratchet candidate:** static check forbidding `InsecureSkipVerify: true` except in `deviceconnection.go` / `tunnel.go` allowlist.

### ENT-010: CI does not gate the products it claims to ship

- **Priority:** P2
- **Dimensions:** Correctness / verification; Build / release / operations
- **Status:** observed fact
- **Evidence:**
  - Windows job builds only; `go test` commented (`.github/workflows/test.yml:41-42`).
  - No macOS unit-test job (macOS is release-only).
  - Root `go test ./...` does not test `restapi` or `ncm` modules.
  - Real-device workflow is `workflow_dispatch` + ngrok + secrets (`.github/workflows/real-device.yml`); not a PR gate.
  - `golangci-lint-action` v9 with linter v2.6 vs local 2.12.2; checkout/setup-go mix v3/v4.
  - Lint config `.golangci.yml` enables only `forbidigo` for `print`/`println`.
- **Mechanism:** Windows regressions, REST compile breaks (ENT-001), and ncm breaks can merge green. “Unit tests” workflow name overstates the gate.
- **Blast radius:** all PRs; Windows CLI users; REST/ncm.
- **Counterevidence checked:** Linux fast tests + gofmt + lint *do* run on PRs. `make lint` exists. Pre-commit hook duplicates that if `core.hooksPath` is set (`Makefile:38-41`).
- **Smallest coherent remediation:** Uncomment Windows fast tests once imagemounter is hermetic; add `restapi`/`ncm` test jobs; one setup-go/checkout version.
- **Verification:** a deliberate `restapi` compile error must fail CI.
- **Ratchet candidate:** required checks for linux tests, windows tests, restapi tests, ncm tests.

### ENT-011: `DeviceConnectionInterface` is satisfied by panicking stubs

- **Priority:** P2
- **Dimensions:** Local code quality; Correctness / verification
- **Status:** observed fact
- **Evidence:**
  - `ios/deviceconnection.go:36-68` `DeviceConnectionRWC` — `Conn`, `DisableSessionSSL`, all four `EnableSessionSsl*` **panic("unimplemented")**. Comment: `TODO: remove the need for this with some refactoring`.
  - Used for iOS 17 tunnel-backed connections (`NewDeviceConnectionWithRWC`). Calling SSL enable on a shim/XPC conn crashes the CLI.
  - Other production panics: `ios/pair_read.go:77` (bad pair plist), `ios/nskeyedarchiver/archiver.go:83`, `ios/dtx_codec/dtxprimitivedictionary.go:55,256`, `ios/debugproxy/muxhandler.go` and `ios/debugproxy/binforward.go` (many `panic` on forward failure), `ios/appservice/appservice.go:240`, `cmd/configure/configure.go:38` `panic(reason)`.
- **Mechanism:** A wide interface was kept so tunnel connections could plug into usbmux-era code. SSL methods are not representable; they panic instead of `return error`. debugproxy treats protocol mistakes as process death.
- **Blast radius:** any iOS 17 path that still hits `EnableSessionSsl*`; debugproxy users; malformed pair records.
- **Counterevidence checked:** SSL handshake-only list (`ios/connect.go:59-64`) is for usbmux DTX services; tunnel connections should not need those methods. The stub exists because the type is shared. Tests do not cover calling SSL on RWC.
- **Smallest coherent remediation:** Split the interface (read/write/close vs SSL session). Return `fmt.Errorf("ssl not supported on tunnel connections")`. Convert debugproxy panics to `log` + disconnect.
- **Verification:** test that `DeviceConnectionRWC.EnableSessionSsl` returns error, not panic.
- **Ratchet candidate:** lint `panic(` in non-test production packages except an allowlist.

### ENT-012: Device middleware continues the request after a 500

- **Priority:** P2
- **Dimensions:** Correctness / verification; Local code quality
- **Status:** observed fact
- **Evidence:** `restapi/api/middleware.go:43-54` — on `deviceWithRsdProvider` error it `c.JSON(500)` then `c.Next()` **without** `return`/`Abort`. Execution always `c.Set(IOS_KEY, device)` (possibly the pre-RSD device) and `c.Next()` again — handlers run after a 500, potentially writing a second body.
- **Mechanism:** iOS 17 REST calls can both error and proceed (pair/install/WDA) on a device object missing RSD. Flaky tunnel attach becomes a dual-response bug.
- **Blast radius:** all `/api/v1/device/:udid/*` routes.
- **Counterevidence checked:** `middleware_test.go` exists but does not cover this branch (restapi/api currently does not compile — ENT-001).
- **Smallest coherent remediation:** `c.AbortWithStatusJSON`; do not `c.Next()` on error.
- **Verification:** httptest: RSD failure yields a single 500 and handler not invoked.
- **Ratchet candidate:** that httptest in `restapi/api`.

### ENT-013: Location simulation has two stacks; CLI and REST pick different ones

- **Priority:** P2
- **Dimensions:** Redundancy / sources of truth; Change amplification
- **Status:** observed fact
- **Evidence:**
  - `ios/simlocation` — `com.apple.dt.simulatelocation` over usbmux only (`ios/simlocation/simlocation.go:18-31`).
  - `instruments.LocationSimulationService` — DTX `LocationSimulation` (`ios/instruments/location_simulation.go:8-44`), which *does* follow `connectInstruments` RSD.
  - CLI `setlocation`: if `SupportsRsd()` use instruments, else simlocation (`main.go:899-913`). `setlocationgpx` / `resetlocation` always simlocation (`main.go:916-926, 2525-2560`).
  - REST `SetLocation` / `ResetLocation` always simlocation (`restapi/api/device_endpoints.go:191-226`, `restapi/api/routes.go:36-38`).
- **Mechanism:** iOS 17 CLI location works via instruments; REST location and GPX/reset stay on the old service. Same user-facing feature, two devices/OS matrices, two failure modes.
- **Blast radius:** `ios setlocation*`, REST `/setlocation`.
- **Counterevidence checked:** Apple replaced the old simulate-location service; keeping both for old iOS is legitimate. The bug is the **split picker**, not the existence of two protocols.
- **Smallest coherent remediation:** One `SetLocation(device, lat, lon)` in the library that matches the CLI RSD branch; REST and GPX/reset call it.
- **Verification:** unit test of the picker with `SupportsRsd` true/false fakes; REST uses the same function.
- **Ratchet candidate:** REST and CLI import the same helper (grep for `simlocation.SetLocation` outside that helper).

### ENT-014: Tunnel agent “wait until ready” does not wait

- **Priority:** P2
- **Dimensions:** Correctness / verification; Local code quality
- **Status:** observed fact
- **Evidence:**
  - `WaitUntilAgentReady` (`ios/tunnel/tunnel_api.go:43-55`): loop; on `Get` **error** returns `false`; on non-200, loops with **no sleep** (busy-wait); 200 ms client timeout (`ios/tunnel/tunnel_api.go:24-26`).
  - `RunAgent` (`ios/tunnel/tunnel_api.go:89-90`) ignores the bool and always `return nil`.
- **Mechanism:** If the child is not listening yet, `Get` errors → “ready” is skipped and callers talk to a down agent. If it listens with a non-200 `/ready`, the parent busy-spins. Tunnel start races are diagnosed as “device not found” further up.
- **Blast radius:** `ios tunnel start` auto-agent, any `RunAgent("user"|"kernel")`.
- **Counterevidence checked:** `IsAgentRunning` health check is used to skip start if already up. Self-heal tests (`selfheal_test.go`) cover `UpdateTunnels` rebuild, not agent spawn. This fork’s HEAD commit is the self-heal — that part *is* tested.
- **Smallest coherent remediation:** Retry on connection error with sleep/timeout; return error from `RunAgent` if not ready by deadline.
- **Verification:** fake HTTP `/ready` delayed 500 ms; `RunAgent` waits, then succeeds; timeout fails the function.
- **Ratchet candidate:** unit test on `WaitUntilAgentReady` with `httptest` (injectable client).

### ENT-015: Dual loggers without a boundary

- **Priority:** P3
- **Dimensions:** Redundancy / sources of truth; Local code quality
- **Status:** observed fact
- **Evidence:** 79 files import logrus; 7 import `log/slog` (notably `ios/tunnel/*`, `ncm`, `ios/discover.go` imports **both**). `ios/tunnel/tunnel.go:22-24` imports logrus twice (`logrus` and `log`).
- **Mechanism:** Tunnel self-heal logs slog; neighboring `UpdateTunnels` uses logrus (`ios/tunnel/tunnel_api.go:300-301`). `-v`/`-t` CLI flags configure logrus only.
- **Blast radius:** operator debugging of iOS 17 tunnels.
- **Counterevidence checked:** ncm is a separate binary (slog-only is fine there). Library tunnel is invoked from the CLI.
- **Smallest coherent remediation:** slog in new tunnel code is acceptable if CLI bridges logrus → slog (or vice versa) at `main`. Do not import both in one file.
- **Verification:** `discover.go` and `tunnel.go` compile with a single logger.
- **Ratchet candidate:** forbid dual import in the same file.

### ENT-016: Linter ratchet is empty except `print`

- **Priority:** P3
- **Dimensions:** Local code quality; Documentation / governance
- **Status:** observed fact
- **Evidence:** `.golangci.yml` `default: none`, enable `forbidigo` only. `golangci-lint run ./...` → 0 issues. CI and `.githooks/pre-commit` both run it, so the gate is green by construction.
- **Mechanism:** A lint job that cannot fail on real issues trains the workflow to look healthy (ENT-010).
- **Blast radius:** false confidence on PRs.
- **Counterevidence checked:** gofmt is a real gate (`.github/workflows/test.yml:66-67`). forbidigo does block builtin `print`.
- **Smallest coherent remediation:** Enable a small set that matches Go norms already mostly followed (`errcheck`, `govet`, `staticcheck`) incrementally; do not dump the default set in one shot.
- **Verification:** a planted `print("x")` fails CI today; a planted unused error should fail after ratchet.
- **Ratchet candidate:** hygiene `quality.lint` once a real set is on.

### ENT-017: Two npm installers, two directory layouts

- **Priority:** P3
- **Dimensions:** Redundancy / sources of truth; Build / release / operations
- **Status:** observed fact
- **Evidence:**
  - `npm_publish/package.json` `postinstall`: `postinstall.js`; `"main": "index.js"`.
  - `npm_publish/postinstall.js:149-156` copies `dist/go-ios-${platform}-${arch}_...`; Darwin arm64 **forces amd64** (“using amd64 build on M1 mac”); `process.platform === "w32"` is dead (should be `win32`).
  - `npm_publish/index.js:36-38` looks up `go-ios-darwin-fat_darwin_fat`.
  - `.github/workflows/release.yml:146-156` publishes **amd64 and arm64** darwin dirs, both copies of the same `lipo` binary — **not** `fat`.
- **Mechanism:** `require('go-ios')` (`index.js`) cannot find the binary the release job just packed; postinstall may still place `ios` on PATH. Darwin arm64 works only because both folders contain the universal binary and postinstall prefers amd64.
- **Blast radius:** `npm install -g go-ios` (README’s primary install).
- **Counterevidence checked:** `index.js` is the more recent, cleaner implementation; postinstall is the one npm actually runs.
- **Smallest coherent remediation:** One installer; release layout matches it (either `fat` or per-arch). Delete the other.
- **Verification:** pack the npm dir as CI does; `node index.js --version` and `postinstall` both find the binary on darwin-arm64 and linux-amd64.
- **Ratchet candidate:** CI job that `npm pack` + runs postinstall in a temp prefix.

### ENT-018: Opaque 15 MB binaries in git without LFS or provenance

- **Priority:** P3
- **Dimensions:** Security / dependencies; Build / release / operations
- **Status:** observed fact
- **Evidence:** `testdata/app-signer-linux` 5.4 MB, `app-signer-mac` 4.9 MB, `wda.ipa` 5.1 MB; `git check-attr filter` unspecified; no `.gitattributes`. Used by `real-device.yml` to sign WDA.
- **Mechanism:** Unauditable executables in the tree are a supply-chain input to device CI. They bloat clones and cannot be rebuilt from this repo.
- **Blast radius:** anyone who clones; the real-device workflow.
- **Counterevidence checked:** `.gitignore` ignores `*.p12`/`*.mobileprovision` (good). WDA IPA is a fixture, not a secret.
- **Smallest coherent remediation:** Document how to rebuild the signers; Git LFS or a release asset with checksums; do not execute unsigned blobs from git on developer machines without that doc.
- **Verification:** checksum file matches; or binaries gone and workflow downloads a tagged artifact.
- **Ratchet candidate:** `git lfs` or a CI step that `shasum` matches a pinned digest.

## Redundancy and competing-source-of-truth inventory

| Concept | Instances | Drift already? | Owner if converged |
|---|---|---|---|
| Device transport | `ConnectToService`, `ConnectToShimService`, `ConnectToServiceTunnelIface`, `ConnectToXpcServiceTunnelIface` | yes — some services never RSD | `ios.Connect` (ENT-002) |
| DTX dial | `NewUsbmuxdConnection` / `NewTunnelConnection` | yes | wrap in instruments-style helper |
| XCUITest session | xcuitestrunner.go / `_11.go` / `_12.go` | yes (TODO protocolVersion) | testmanagerd orchestrator (ENT-004) |
| File access | AFC, house_arrest+AFC, fileservice XPC; CLI `fsync` vs `file` | yes — iOS 17 vs legacy | keep two protocols, one CLI |
| Location | simlocation vs instruments; CLI vs REST | yes (ENT-013) | one library picker |
| App list/launch | installationproxy vs appservice | iOS 17 XPC vs lockdown | document as era split |
| Image mount | `imagemounter.go` vs `personalized_image_mounter.go` | era split | keep; share HTTP/TSS client |
| Version string | `main.go` const, `version.txt`, npm `package.json` | yes (ENT-007) | ldflags |
| Logger | logrus vs slog | yes (ENT-015) | one at process edge |
| npm binary layout | postinstall.js vs index.js vs release.yml | yes (ENT-017) | one |
| go-ios module pin | restapi `v1.0.91` vs replace `../` vs tags `v1.0.213` | yes (ENT-001) | workspace replace only, require HEAD |
| Default HTTP agent port | comment 60106 vs const 60105 | comment lie | fix comment |
| Lockdown a11y toggles | `lockdown_assistivetouch.go`, `_voiceover.go`, `_zoomtouch.go`, `_time_format.go` | parallel files, same shape | acceptable duplication (tiny, stable) |

**Deliberate duplication (not defects):** lockdown value wrappers; platform files `tunnel_{darwin,linux,windows}.go`; Xcode-era XCTest *protocol* differences (the copy-paste of orchestration *is* the defect).

## Healthy structure worth retaining

- **Hub-and-spoke `package ios`.** Leaves depend on `ios`; `ios` depends only on `ios/http` and `ios/xpc`. `go list` shows **no import cycles**. Fan-in is honest: usbmux/lockdown/pair/RSD belong in one place.
- **`ios/tunnel/doc.go`** is the best architecture document in the repo — pairing, SRP, QUIC, RSD — and matches the code.
- **Codec packages with fixtures:** `dtx_codec`, `nskeyedarchiver`, `opack`, `xpc`, `plistcodec` have table tests against captured binaries. That is the right oracle for reverse-engineered protocols.
- **Era-aware `New()` in syslog, zipconduit, ostrace, instruments.** Pattern to copy (ENT-002).
- **Tunnel self-heal (this branch).** `Tunnel.done` / `IsAlive` (`ios/tunnel/tunnel.go:39-61`), `deathWatchConn` (`ios/tunnel/userspace_tunnel.go:198-258`), rebuild in `UpdateTunnels` (`ios/tunnel/tunnel_api.go:290-304`), **injectable** `deviceLister`/`tunnelStarter` and `TestUpdateTunnels_RebuildsDeadTunnel` (`ios/tunnel/selfheal_test.go:73`). This is how the rest of tunnel/agent should be tested.
- **NCM isolation.** `ncm` does not import `ios`; Linux USB/TAP is a separate deployable. Keep that cut; only fix the module path (ENT-008).
- **Pair-record TLS for usbmux** (client cert from pair record) is the right model; do not “fix” it into public PKI.
- **`//go:build !fast` on real device tests** is the right split — apply it to imagemounter (ENT-005).
- **MIT LICENSE, README feature map, CONTRIBUTING CoC.** gofmt is clean.

## Hygiene posture

`hygiene.yaml` is **absent**. Hygiene posture is **not declared**. The validator was not run (skill: do not initialize unless asked).

Overlap with entropy (what hygiene *would* catch once declared):

| Would-be item | Reality now |
|---|---|
| `correctness.unit-tests` | Linux `go test -tags=fast` exists but is not hermetic (ENT-005); Windows disabled |
| `correctness.restapi` | does not compile (ENT-001) |
| `quality.fmt` | **held** — `gofmt -l .` empty, CI step present |
| `quality.lint` | job exists, config is forbidigo-only (ENT-016) |
| `security.vuln-scan` | absent (`govulncheck` not installed, no CI scanner) |
| `release.version` | sed of source (ENT-007) |
| `docs.readme` | present |
| `governance.license` | MIT present |
| `vcs.default-branch` | remote `main` (upstream convention; this fleet usually wants `master` — owner call) |

Entropy findings suitable as future hygiene ratchets: ENT-001 CI restapi build, ENT-005 networkless fast tests, ENT-008 `GOWORK=off` build, ENT-009 TSS TLS allowlist, ENT-016 lint set.

## Oracle coverage and residue

| Property | Decided by |
|---|---|
| DTX encode/decode, NSKeyedArchive, opack, XPC, plist | shipped unit tests + fixtures |
| USBMux list/parse, pair plist roundtrip (happy path) | unit tests |
| Tunnel self-heal rebuild | shipped unit test (`selfheal_test.go`) |
| ostrace filters, xctestrun parse, test listener | shipped unit tests |
| AFC walk (non-device) | unit tests |
| CLI docopt parse | `main_test.go` (limited) |
| `gofmt` | CI |
| `print`/`println` forbidden | golangci-lint |
| REST compiles / serves | **nothing** (broken) |
| NCM in root CI | **nothing** |
| Windows tests | **nothing** (commented) |
| iOS 17 connect matrix per service | **nothing** (manual / live) |
| Live install / tunnel / XCTest / WDA | `real-device.yml` manual ngrok; `!fast` tests not in CI |
| TSS TLS verification | **nothing** |
| npm postinstall layout | **nothing** |
| Vulnerability inventory | **nothing** (`govulncheck` missing) |
| imagemounter download | live network (anti-oracle) |

**Owner residue (intent, not more grep):**

1. Is `restapi` a product this fork will keep, or should it be dropped from `go.work` until it compiles?
2. Should this fork ever run `.github/workflows/release.yml` / npm publish, or is that upstream-only?
3. Fleet default branch `master` vs this repo’s inherited `main`.
4. Accept SHA-1 lockdown pairing certs as protocol forever? (Recommended: yes.)
5. Keep AFC and fileservice as two user-facing commands, or unify `ios file`?

Failed/skipped checks: restapi build/test; imagemounter tests; full `go test -tags=fast ./...` (timeout); govulncheck; live device journeys; hygiene validator (undeclared).

## Remediation sequence

1. **Oracles first.** Tag imagemounter network tests `!fast`; fix WaitGroup; restore `http.DefaultTransport`. Add `restapi` and `ncm` `go test`/`go build` jobs. Fix `app_endpoints.go` accessors and swagger generation so REST compiles (ENT-001, ENT-005, ENT-010).
2. **Stop the 500-and-continue and nil-close landmines** (ENT-012, ENT-003 `processList`). Abort REST middleware; defer Close only on non-nil.
3. **Converge connection selection.** Library `Connect` + migrate afc/installationproxy/accessibility/simlocation (ENT-002, ENT-013). One location helper used by CLI and REST.
4. **Tunnel agent wait** returns error on timeout (ENT-014). TSS uses default TLS (ENT-009).
5. **XCTest orchestration extract** after connect helper exists (ENT-004) — otherwise the extract still forks on dial.
6. **CLI split** of `main.go` by domain (ENT-003) only after usage/tests can move with the functions.
7. **NCM module path** (ENT-008); **ldflags version** (ENT-007); **one npm installer** (ENT-017).
8. **Ratchet** the compile/test jobs and networkless fast tests into CI; declare `hygiene.yaml` only when those gates exist (do not author it in this audit).
9. Re-run this audit on the same definitions.

Do not rewrite the library into “clean architecture.” The hub-and-spoke is the asset.
)
