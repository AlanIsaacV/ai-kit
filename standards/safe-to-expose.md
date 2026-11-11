# Safe to expose — what a trii app meets before the internet reaches it

Read by the `audit` skill once it exists, and by any agent or person asked whether a
repository is fit to expose; nothing loads it automatically yet. The premise: **the
serious gaps in trii's apps are logic flaws — who gets in, and untrusted input
reaching something powerful — and a read guided by six ranked axes finds them where
scanners do not.** The axes are in the order they bite.

## How to apply it

- Read the repository at one commit and only read it (`git show <commit>:<path>`,
  `git grep`). Never check it out, write to it, run the app, or contact any
  endpoint it serves or calls.
- Exposure is the premise: every route the app serves is reachable from the
  internet unless the repository declares otherwise, and a reverse proxy that
  forwards every path changes nothing.
- Work through `## Checklist` in order. Each item ends as met, as a finding with
  `path:line` and a severity, or as not assessable with the reason.
- A finding that meets the Iron Law is **confirmed**; one that does not is
  **suspected**, rated NEEDS REVIEW. Report the two apart, decide the verdict with
  `## The fit-to-expose bar`, and list what was not read.

## Severity

Ported from ai-marketplace's `security` plugin, which this standard does not need
installed. Line ranges are at its commit `1af91f1` (`git show 1af91f1:<path>`):

- `/srv/projects/trii/core/ai-marketplace/plugins/security/skills/security/SKILL.md`
  — the Iron Law (lines 14-20) and the three confirm-exploitability steps (124-132).
- `/srv/projects/trii/core/ai-marketplace/plugins/security/skills/security/references/sk2-and-exposure.md`
  — the severity rubric, CRITICAL / HIGH / MEDIUM / NEEDS REVIEW (lines 181-190).
- `/srv/projects/trii/core/ai-marketplace/plugins/security/skills/security/references/checklist.md`
  — the per-item severities the axes start from (lines 1-74): object-level
  authorization (7-12), secrets (14-21), injection (23-28), SSRF and header trust
  (30-35), sensitive logs (37-40), dependency advisories (42-58), CORS (60-65).

Left out: the `sk2` gateway map, the fix phase and the Dependabot advice. In the
plugin a route is exposed when a gateway route covers it and the missing control is
`sk2`; here it is exposed when the app serves it, and the missing control is what
the axis names. The plugin covers Python and Go; this adds Node and MCP servers.

### The Iron Law

```
NO "CRITICAL" WITHOUT A CONFIRMED EXPOSURE PATH + A CONFIRMED MISSING CONTROL
```

A standard that cries wolf is worse than none: people stop reading it. A finding
rated as exploitable rests on two facts the reader verified in the code:

1. **The exposure path.** A chain traced from a route the app serves — an HTTP
   route, an MCP tool, a webhook, or a job such a route enqueues — to the sink,
   with the role a caller needs at the route. The role is read off the code; a
   restriction configured outside the repository does not leave it unconfirmed
   (see `## The fit-to-expose bar`).
2. **The missing control.** The handler, its router or dependency, any global
   middleware and the sink itself were read, and the guard is not there.

If only one is confirmed, the finding is NEEDS REVIEW, naming the missing fact.
Write the exploit in one sentence — who can do what, by sending which request; if it
cannot be written from evidence, it is not Critical yet. This standard holds HIGH to
the same two facts, because the bar counts only confirmed findings.

### The rubric

| Severity | Condition |
|---|---|
| CRITICAL | Reachable by a caller with no login, and reaches another user's data, the app's secrets, a command or code run on the host, or a fetch inside the network. Or a login that fails open when its configuration is missing, one unset variable on the VM away; that finding names the sinks it opens, each rated by the login its own route requires. Or a committed secret that is live and opens a production system. Or an object-level gap on money or PII. |
| HIGH | The same sinks, reachable by any logged-in user: another user's data or files, injection, SSRF, a page the server renders running the user's script. Or the host's sinks — its files and secrets, a command or code run on it, a fetch inside the network — reachable only by an admin role: no role in the app is meant to hold the host. Or a confirmed secret or injection with a real path to exploit. |
| MEDIUM | A real weakness with limited or indirect impact: it needs the server already compromised (a plaintext dump no route serves, whatever it holds), it leaks hashes rather than secrets, sensitive data in logs, permissive CORS, a vulnerable dependency off any exposed path, a root container, unbounded resource use behind the login. |
| NEEDS REVIEW | The Iron Law is not met: the path or the missing control could not be confirmed. Name the missing fact. |

Severity follows confirmed impact. A sink is rated by the login its own route
requires; a login that fails open when configuration is missing is its own finding,
rated as if it were missing. A login method shipped switched off, on once a variable
is set, is rated as if on and counts toward M1 and M2. A sink that only another
confirmed finding reaches, a backup behind a file-read flaw or a secret file alike,
is rated in that finding, not raised on its own. Never inflate to be safe, never
bury a confirmed hole. Two readers of one commit must agree on every Critical and
High: rate by the table's condition, not by how alarming the code looks, and when
torn, rate at what the confirmed facts support and name the fact that would move it.

## The fit-to-expose bar

An app is **fit to expose** when both hold:

1. No confirmed Critical or High finding. Suspected findings do not block, but each
   is listed with the fact that would confirm it.
2. All six mandatory controls are present. A control is present only when the
   reader found it in the code or the repository's deploy files. Not found is
   absent.

An access restriction configured only outside the repository — an OAuth consent
screen and its tester list, a firewall, IAP, where a backup is kept, any console
setting — is not a control, whatever the repository's docs say of it. A finding
it would narrow is rated by what the code allows and keeps that severity, and
names the restriction in one line as a mitigation outside the repository that was
not verified. Every item it bears on, a mandatory control's included, is rated on
what the repository shows.

| Control | Present when | Items |
|---|---|---|
| M1. Login fails closed when its configuration is missing | With any login setting missing or empty, the app refuses to start or refuses every request; no code path gives a caller it could not identify a user or a role. | 1.1 |
| M2. Google login enforces `hd`, `email_verified` and a nonce | The ID token's claims are checked: `hd` equals trii's Workspace domain and a token without `hd` is rejected; `email_verified` is true; the nonce is the one this session sent. An email suffix is not `hd`. A Google login shipped switched off is judged on its code. | 1.2 |
| M3. No committed secret | The tree at the commit holds no credential and no real secret as a default, and no live secret was found in history. | 3.1 |
| M4. The container does not run as root | Every Dockerfile's final stage runs as a non-root user. An app deployed without a container runs under its own non-root service user. | 4.1 |
| M5. User-supplied outbound URLs are filtered, IPv4-mapped IPv6 forms included | Every fetch of a URL the code did not write goes through one filter, and the filter rejects internal addresses in every form, redirects included. An app that fetches no such URL meets it by having none. | 2.5, 2.6 |
| M6. The bind address and the exposure are declared | The repository states what the app binds and what reaches it from the internet, and the code binds what is stated. | 4.2 |

The verdict is either "fit to expose", or "not fit to expose" followed by every
confirmed Critical and High and every absent control.

### Not covered by the bar

Fit to expose says nothing about the cloud account, the network or the machine the
app lands on. These areas of
`/srv/projects/trii/scripts/security/diagnostico-2026-09-18.md` are checked against
the live systems, not the repository: firewall rules and VMs reachable from the
internet (2.1); Cloud SQL network, public IPs, SSL mode and settings (2.2, 3.5);
plaintext secrets in Cloud Run environment variables (2.4); IAM owners, keys and
roles, and the default Compute service account every service shares (3.4, 2.5);
Cloud Armor coverage (3.1); TLS policy at the edge (3.6); detection (3.7); GitHub
organisation settings such as rulesets, 2FA and secret scanning (3.3); registry
scanning of images (3.8); and the host itself (3.9). Nor does it cover OAuth
consent screens and their tester lists, or any other setting kept in a console:
findings they would narrow stay rated by the code, as the bar says.

Not settled in this version, and so not assessable with the reason: a finding on
two axes, a stack other than the three below, an app public by design with no login.

## Scanners are evidence, not the read

ai-kit's `evidence` (`tools/evidence`) runs gitleaks 8.30.1, osv-scanner 2.7.0 and
govulncheck v1.8.0, plus three regex rules over workflows: an unpinned `uses:` or
image, `credentials_json`, and `pull_request_target`. For review it reports what one
change introduced; for the audit, `--tree` reports all it finds in one exported tree,
except git history and what gitleaks skips by default. No logic flaw ranked
first here falls under its rules, and a check not run or partial is not a clean result.

## 1. Who gets in

Whether the app knows who is calling, refuses whoever it should, and keeps each
caller to what is theirs. A route with no login that reaches data is CRITICAL; a
logged-in user reaching another user's data or files is HIGH.

- **Login fails closed (M1).** The dangerous shape is a branch that, with the login
  configuration absent, treats the caller as a local user with full rights because
  the app "must be on someone's laptop". One missing variable on the VM opens it to
  the internet; the app must refuse to start or refuse every request instead. A
  bypass switch (`DEV_AUTH_BYPASS`, a test login) the deployed configuration can
  turn on is the same shape.
- **Google login (M2).** The `hd` parameter in the authorization URL only hints the
  account picker. Check the ID token's claims: `hd` present and equal to trii's
  domain, `email_verified` true, and the nonce stored with the session. Comparing
  `hd` only when present, or reading the email's suffix, admits a non-Workspace
  Google account carrying a trii address.
- **Default deny.** Per-route authentication depends on every new route remembering
  it. Apply it to everything and opt public routes out by name, or keep a route
  inventory. Missing both is MEDIUM; each route left unprotected is its own finding.
- **Routes outside the login.** Webhooks, OAuth callbacks, connector and agent
  endpoints and internal endpoints each need their own authentication: a provider
  signature or HMAC over the raw body compared in constant time, or a shared
  secret. Behind a reverse proxy on the same host every request arrives from
  loopback, so "only from localhost" is no control there.
- **Object level.** The resource a caller names — an id, a key, a file path — is
  checked against the identity the session resolved, never one the request
  carries. A path is resolved before it is compared: a check that splits the raw
  string on `/` and approves `files/<mine>/../../<theirs>` is a traversal.
- **Sessions.** Cookies HttpOnly, Secure over https, SameSite Lax or Strict; a
  session secret with no default; logout that ends the session on the server; an
  Origin check or token on state-changing routes.

**Node.** Raw `node:http` has no middleware layer: a branch handled before the
identity lookup is public. In Next.js, `middleware.ts` runs only where its matcher
says, and every server action is a public POST endpoint that checks the session
itself. A hand-rolled OIDC callback is acceptable when the token comes straight from
Google's token endpoint, but every claim check is then the code's own: look for
`hd`, `email_verified` and `nonce` by name. Paths: `path.resolve`, then
`startsWith(root + path.sep)`.

**Python/FastAPI.** `Depends(...)` per route is the per-handler shape;
`APIRouter(dependencies=[...])` or an app-level dependency is the default-deny
shape, and a router included without it is a gap. With Authlib, the user info it
returns still has to be checked for `hd` and `email_verified` explicitly; an
`endswith("@trii.co")` is not that check. Starlette's `SessionMiddleware` signs with
`itsdangerous`: look for a secret with a development default and `https_only` left
at its default of off. Paths: `Path.resolve()` and then `is_relative_to(root)`.

**Go, MCP servers included.** Look for routes on the bare mux outside the group the
auth middleware wraps. go-oidc's verifier checks signature, issuer, audience and
expiry, not nonce, `hd` or `email_verified`: compare those after `Verify`. scs
defaults the cookie's Secure flag off. `filepath.Clean` is not containment; use
`filepath.Rel` or `os.Root`. An MCP server over HTTP verifies the caller's bearer —
such as a cerberus-gw EdDSA assertion — on every call and takes the acting user from
it, never from a tool argument; over stdio it has no network caller.

## 2. Untrusted input reaching something powerful

Untrusted is anything the app's own code did not write: request fields, uploads,
pasted links, third-party API responses, and model output, which carries whatever
the pages the model read told it. Powerful is a shell, SQL, a browser, an
interpreter, the filesystem, an outbound fetch, rendered HTML. By its route's login:
none, CRITICAL; any logged-in user, HIGH; only through the app's own server, MEDIUM.

- **Shells.** Start processes from an argv array with no shell. On Windows a
  `.cmd` or `.bat` target always runs through `cmd.exe`, which reinterprets every
  argument. A value the app's server sends to a program on someone's computer is
  untrusted there: whoever controls the server controls the command.
- **SQL.** Values as bound parameters only. An identifier built into a statement
  comes from an allowlist and the driver's quoting.
- **Headless browsers.** A server-side Chrome rendering what a user, a file or a
  model influenced is a code-execution sink. It runs without
  `--allow-file-access-from-files` or `--no-sandbox`, loads no `file://` page built
  from that data, and gets user text only as data (page arguments, `textContent`,
  `JSON.stringify`), never spliced into source. Stripping quotes is not escaping: a
  template literal's `${` still runs. A splice is a confirmed missing control once
  the reader names where it runs: a string or template literal in a script the
  renderer loads at that commit, which a caller can make the splice replace — any
  string a caller-chosen key selects, not only those the UI offers — and where what
  the filter leaves still runs. Rate it then by who reaches the route; with no such
  position named, NEEDS REVIEW.
- **Client-supplied markup.** No route accepts HTML, a script, a template or a
  catalog entry from the client that the server later renders or runs. When one
  route filters what reaches the renderer and another passes the client's object
  through, the second is the finding.
- **Outbound fetches (M5).** Every fetch of a URL the code did not write, URLs out
  of a model's answer included, goes through one function; grep every `fetch`, HTTP
  client and browser navigation to show none skips it. It allows http(s) only,
  resolves the host, and rejects loopback, private, link-local (cloud metadata at
  169.254.169.254), CGNAT, unique-local and multicast addresses, and IPv4-mapped
  IPv6 dotted (`::ffff:127.0.0.1`) and hex (`::ffff:7f00:1`), again on every
  redirect. Re-resolving instead of dialing the checked address leaves DNS
  rebinding: MEDIUM.
- **Escaping and eval.** Templates escape by default; every escape hatch receives
  only sanitized content. Nothing from input reaches `eval`, `new Function`, `vm`,
  `pickle` or an unsafe YAML loader.
- **Files from users.** Uploads, archives and code to parse are bounded in size
  and time, extracted only to paths checked to stay in the target directory, and
  handed to tools with fixed arguments.

**Node.** `child_process.spawn` with `shell: true`, `exec`, and `execSync` with a
template string. puppeteer's `launch` arguments, `page.goto('file://…')` and
`setContent` with interpolated text. The WHATWG URL parser serializes any
IPv4-mapped literal to hex — `new URL('http://[::ffff:127.0.0.1]/').hostname` is
`[::ffff:7f00:1]` — so a filter that recurses only on the dotted form never fires on
a URL. Drizzle's `sql` tag binds values, `sql.raw` does not; postgres.js
`sql.unsafe` does not. `dangerouslySetInnerHTML` in React.

**Python/FastAPI.** `subprocess` with `shell=True`, `os.system`. SQLAlchemy
`text(f"…")` with any variable in it. `httpx` or `requests` on a URL from input.
Jinja2 `|safe` and `Markup`; on a Vue frontend, `v-html` fed anything but
DOMPurify's output. `pickle.loads`, `yaml.load` without `SafeLoader`,
`zipfile.extractall`.

**Go, MCP servers included.** `exec.Command("sh", "-c", …)` with a variable; a
constant argv, like a fixed `git` call, is the safe shape. `fmt.Sprintf` into a
query is safe only behind an allowlist plus `pgx.Identifier.Sanitize`. A
`net.Dialer` whose `Control` checks the address dialed covers redirects and
rebinding at once. `template.HTML` and `templ.Raw` bypass autoescaping. An MCP
tool's arguments are chosen by a model: contain paths, fix commands, and bound
parsers run over untrusted code, tree-sitter included.

## 3. Secrets and sensitive data

What the app knows that an attacker wants, and every place a copy lands. A live
secret opening production is CRITICAL, other live committed secrets HIGH.

- **Committed secrets (M3).** No `.env`, service-account JSON, private key, keystore
  (`.key`, `.pem`, `.p12`), token or DSN with a password in the tree, and no real
  secret as a settings default. A template such as `.env.example` holds placeholders
  the app never runs with. A secret removed from the tree is still in history and
  still has to be rotated.
- **Secrets at runtime.** trii's convention is a `.env` on the VM readable only by
  the service user. No secret ends up in an image layer, a variable bundled for the
  browser, a log line, or a child process's environment that does not need it.
- **Stored credentials.** Passwords with a slow hash (scrypt, argon2, bcrypt);
  session and API tokens stored as hashes.
- **Copies.** Backups and exports are listed: where, who reads them, encrypted or
  not, what they hold. A plaintext dump is MEDIUM whatever it holds unless a route
  returns it or its files directly, not through another finding.
- **Logs and errors.** No token, cookie or password in logs, no more personal data
  than the app needs, no stack trace returned to the client.

**Node.** Variables prefixed `NEXT_PUBLIC_` or `VITE_` are compiled into the browser
bundle. `spawn(…, { env: { ...process.env } })` hands every secret to the child.
Logging a whole request body on an auth route.

**Python/FastAPI.** A pydantic `Settings` field with a real-looking default for a
secret, which the app runs with when the variable is unset. `DEBUG` tracebacks in
responses.

**Go, MCP servers included.** Nothing distinct beyond the general rules; reading a
secret from a `*_FILE` path is a good shape to keep.

## 4. Where it runs and who owns it

What a takeover can touch, what the internet reaches, and whether trii rather than
one person holds the keys. Mostly MEDIUM; two of its controls are mandatory.

- **Not root (M4).** A root process turns any code execution into the host. A
  server-side Chrome that needs `--no-sandbox` is a sign the process is root.
- **Bind and exposure declared (M6).** The code binds `127.0.0.1` or a container
  network and the repository says what fronts it — a reverse proxy, TLS, which
  paths — or it binds every interface on purpose and says what restricts it. A
  port with no host binds every interface.
- **Forwarded headers.** `X-Forwarded-For` and kin are trusted only from the proxy.
- **Development modes off.** Debug flags, auto-reload, development servers and
  interactive API docs are off in the deployed configuration unless meant.
- **Ownership.** Repository, domain, VM, OAuth client and mail sender belong to trii
  accounts with more than one owner. A personal VPS, domain, GitHub repository or
  Gmail SMTP seen in the remotes or deploy docs is MEDIUM: it leaves with the
  person.

**Node.** `server.listen(port)` with no host binds every interface, IPv6 included.
`next start` without `-H` does the same. `next dev` in a deploy script.

**Python/FastAPI.** `uvicorn --host 0.0.0.0` in a Dockerfile `CMD`, `--reload` in
production, and `--forwarded-allow-ips '*'`. FastAPI serves `/docs` and
`/openapi.json` unless turned off.

**Go, MCP servers included.** `http.ListenAndServe(":8080")` binds every interface;
a server that refuses an address with no host is the safe shape. An MCP server over
stdio binds nothing; over HTTP it binds like any other server.

## 5. Supply chain

Code the app runs that nobody at trii wrote, and the pipeline that ships it. A
workflow holding a production credential that runs a third-party action by tag is
HIGH: whoever moves the tag runs code with the key. The rest is MEDIUM.

- **Lockfiles.** Committed, and installs use them.
- **Advisories.** The stack's scanner runs over the lockfile, and only advisories
  on a path the app uses are findings. The report says which scanner ran and which
  stack had none.
- **Workflows.** Third-party actions pinned by full commit SHA. Cloud access
  through Workload Identity Federation, not a static key in `credentials_json`. No
  `pull_request_target` job that checks out or runs the pull request's code. A
  `permissions:` block at the minimum.
- **Images.** Base images pinned by digest: no `latest`, no tag alone.

**Node.** `package-lock.json` with `npm ci`; `npm install` in a deploy script
resolves fresh, and `postinstall` scripts run. osv-scanner reads Node lockfiles.

**Python/FastAPI.** `uv.lock` with `uv sync --frozen`, or requirements with hashes;
a bare `pip install` of names resolves fresh. osv-scanner reads `uv.lock` only when
the root Dockerfile runs `uv sync`.

**Go, MCP servers included.** `go.sum` committed. govulncheck is call-graph aware,
so what it reports is reached. A `replace` directive to a fork is a dependency
nobody reviews.

## 6. Resources and cost

Whether one caller can exhaust the machine or the bill. Unbounded heavy or paid
work reachable with no login is HIGH; behind the login, MEDIUM.

- **Bounds on input.** Request bodies and uploads have a size limit; the server has
  read and write timeouts.
- **Bounds on work.** Every outbound call and child process has a timeout; heavy
  work — browsers, ffmpeg, model calls — has a concurrency limit.
- **Bounds on spend.** Login attempts, code-sending mail and paid operations have
  per-user rate limits or quotas, and their cost can be attributed to a user.
- **Bounds on disk.** Generated files and temporary data are cleaned up or kept
  under a retention rule.

**Node.** Raw `node:http` reads bodies with no limit unless the code imposes one;
check the helper that collects the body. Next.js caps server-action bodies, not
route handlers that read the stream.

**Python/FastAPI.** FastAPI imposes no body limit; uvicorn has
`--limit-concurrency`. An `UploadFile` is spooled to disk with no cap of its own.

**Go, MCP servers included.** `http.ListenAndServe` sets no timeouts; use an
`http.Server` with them and `http.MaxBytesReader`. `exec.CommandContext` with a
deadline. MCP tools that scan a whole repository are bounded in files and time.

## Checklist

Each item is checked against the repository at one commit. Items marked M1 to M6
decide the bar's mandatory controls; the section that explains an item is in
brackets. A `Scanner:` note names the `evidence` check that touches the item; an
item without one has no scanner evidence.

- [ ] 1.1 (M1) With any login setting missing or empty — client id or secret,
  allowed domain, session secret, public URL — the app refuses to start or refuses
  every request; no code path gives an unidentified caller a user or a role.
  [1. Who gets in]
- [ ] 1.2 (M2) Google login checks the ID token's claims: `hd` equal to trii's
  Workspace domain, a token lacking `hd` rejected; `email_verified` true; the nonce
  this session sent. [1. Who gets in]
- [ ] 1.3 No login bypass — a development flag, a test login, a local mode — can be
  on in the deployed configuration. [1. Who gets in]
- [ ] 1.4 Authentication covers every route by default, public routes opted out by
  name, or the repository keeps an inventory of routes and the auth each requires.
  [1. Who gets in]
- [ ] 1.5 Every route outside the user login — webhook, callback, connector,
  internal endpoint — has its own authentication: a signature or HMAC over the raw
  body, or a shared secret; a loopback check behind a proxy is none.
  [1. Who gets in]
- [ ] 1.6 Every resource a caller names is checked against the identity the session
  resolved, never one from the request; roles are enforced on the server.
  [1. Who gets in]
- [ ] 1.7 A path or key is authorized on its resolved form, checked to stay inside
  the owner's root; a check on the raw string, such as splitting it on `/`, is a
  finding. [1. Who gets in]
- [ ] 1.8 Session cookies are HttpOnly, Secure over https, SameSite Lax or Strict;
  the session secret has no default; logout ends the session server-side;
  state-changing routes reject cross-site requests. [1. Who gets in]
- [ ] 1.9 An MCP server over HTTP verifies the caller's bearer on every call and
  takes the acting user from it, never from a tool argument. [1. Who gets in]
- [ ] 2.1 No process starts through a shell with a variable in its command line —
  `shell: true`, `shell=True`, `sh -c`, a `.cmd`/`.bat` target — values from the
  app's own server included. [2. Untrusted input reaching something powerful]
- [ ] 2.2 SQL takes values only as bound parameters, identifiers only from an
  allowlist with the driver's quoting.
  [2. Untrusted input reaching something powerful]
- [ ] 2.3 A headless browser rendering what a user, file or model influenced runs
  without `--allow-file-access-from-files` or `--no-sandbox`, loads no `file://`
  page built from it, and gets user text only as data, never spliced into source.
  [2. Untrusted input reaching something powerful]
- [ ] 2.4 No route accepts HTML, script, a template or a catalog entry from the
  client that the server later renders or runs; every route into a renderer applies
  the same filter. [2. Untrusted input reaching something powerful]
- [ ] 2.5 (M5) Every fetch or browser navigation to a URL the code did not write —
  from a request, a file, an API or a model's answer — goes through one outbound
  filter. [2. Untrusted input reaching something powerful]
- [ ] 2.6 (M5) That filter allows only http(s), rejects loopback, private,
  link-local, CGNAT, unique-local, multicast and IPv4-mapped IPv6 in dotted and hex
  form, and checks again on every redirect.
  [2. Untrusted input reaching something powerful]
- [ ] 2.7 Templates escape by default; every escape hatch receives only sanitized
  content. [2. Untrusted input reaching something powerful]
- [ ] 2.8 No input reaches `eval`, `new Function`, `vm`, `pickle` or an unsafe YAML
  loader. [2. Untrusted input reaching something powerful]
- [ ] 2.9 Uploads, archives and code to parse are bounded in size and time and
  extracted only inside their target directory.
  [2. Untrusted input reaching something powerful]
- [ ] 3.1 (M3) The tree holds no `.env`, service-account JSON, private key,
  keystore, token, DSN with a password or real secret as a settings default; history
  was scanned for live secrets, or the report says not. Scanner: gitleaks, the
  tree but not its history. [3. Secrets and sensitive data]
- [ ] 3.2 No secret is baked into an image layer, shipped to the browser, logged, or
  passed whole to a child process that does not need it.
  [3. Secrets and sensitive data]
- [ ] 3.3 Passwords are stored with a slow hash, session and API tokens as hashes.
  [3. Secrets and sensitive data]
- [ ] 3.4 Every backup and export is encrypted or holds nothing usable; a plaintext
  dump is MEDIUM whatever it holds unless a route returns it or its files directly,
  not through another finding. [3. Secrets and sensitive data]
- [ ] 3.5 Logs and error responses carry no token, cookie, password or stack trace,
  and no more personal data than needed. [3. Secrets and sensitive data]
- [ ] 4.1 (M4) Every Dockerfile's final stage runs as a non-root user; an app
  deployed without a container runs under its own non-root user.
  [4. Where it runs and who owns it]
- [ ] 4.2 (M6) The repository states the address the app binds and what reaches it
  from the internet, and the code binds what is stated.
  [4. Where it runs and who owns it]
- [ ] 4.3 Forwarded headers are trusted only from the proxy's address.
  [4. Where it runs and who owns it]
- [ ] 4.4 Debug flags, auto-reload, development servers and interactive API docs are
  off in the deployed configuration. [4. Where it runs and who owns it]
- [ ] 4.5 The repository, domain, VM, OAuth client and mail sender belong to trii
  accounts with more than one owner. [4. Where it runs and who owns it]
- [ ] 5.1 A lockfile is committed and every install uses it. [5. Supply chain]
- [ ] 5.2 Advisories were checked with the stack's scanner, and the report names any
  stack that had none. Scanner: govulncheck, osv-scanner (Rust, JavaScript,
  Python); a manifest with no lockfile the deploy uses is not run. [5. Supply chain]
- [ ] 5.3 Every third-party action in a workflow is pinned by full commit SHA.
  Scanner: workflow rule. [5. Supply chain]
- [ ] 5.4 Workflows reach the cloud without a static key (`credentials_json`), run
  no pull request code under `pull_request_target`, and set minimal `permissions:`.
  Scanner: workflow rules for the first two. [5. Supply chain]
- [ ] 5.5 Base images are pinned by digest, no `latest` and no tag alone. Scanner:
  image rule over workflow and action files, not Dockerfiles. [5. Supply chain]
- [ ] 6.1 Request bodies and uploads have a size limit; the server has read and
  write timeouts. [6. Resources and cost]
- [ ] 6.2 Every outbound call and child process has a timeout; heavy work has a
  concurrency limit. [6. Resources and cost]
- [ ] 6.3 Login, code-sending mail and paid operations have per-user rate limits or
  quotas. [6. Resources and cost]
- [ ] 6.4 Generated and temporary files are cleaned up or kept under a retention
  rule. [6. Resources and cost]

## Calibration

The proof that the checklist catches what is already known to exist, at the
severity each finding earns. The rest of the standard stands without this section:
a reader testing the checklist on a repository is given the standard without it.
lab-trii-contenido anchors are paths at `98d78fd` (`origin/mejoras-lab-servidor`),
read with `git show 98d78fd:<path>`; diagnostic anchors are line ranges of
`/srv/projects/trii/scripts/security/diagnostico-2026-09-18.md`. lab's routes sit
behind Caddy on a public domain, as its deploy guide says (`DESPLIEGUE.md:17-19,38`).

| # | Finding | Origin anchor | Axis | Item | Severity |
|---|---|---|---|---|---|
| 1 | Chrome launched with `--allow-file-access-from-files` on a `file://` page, with a user's free text spliced into quoted and template literals; the creator picks both the string replaced and its replacement, and the filter strips only `<`, `>` and the quote | `motor/src/motions.mjs:211,279-285,301-302`, `motor/src/marca/motion.mjs:105,108`, `motor/src/lab.mjs:1301,1316-1320`, `motor/src/serie.mjs:302,321`; where it runs: the catalog motion `m-mosaico`'s template literal (`motor/src/marca/base.mjs:353`), replaced by `${…}` that reads a file; the service user can read `/etc/lab-trii.env` (`DESPLIEGUE.md:224,237`) | 2 | 2.3 | HIGH: any logged-in creator, on their own series, runs script that reads the server's files, the env file included, into their video |
| 2 | `/api/editar` takes the client's `motions` object as the motion catalog, HTML included, and renders it in server Chrome | `motor/src/lab.mjs:1415`, `motor/src/lab.mjs:810`, rendered at `motor/src/lab.mjs:722-744` | 2 | 2.4 | HIGH: any logged-in user runs script in the server's browser |
| 3 | `png_urls` from the model's answer fetched with a bare `fetch`, skipping the outbound filter, and served back | `motor/src/lab.mjs:952-960` | 2 | 2.5 (M5) | HIGH: SSRF with the response readable, by a logged-in user |
| 4 | `ipInterna` recurses only on the dotted `::ffff:a.b.c.d` form; the hex form, which Node's URL parser always produces, reads as public | `motor/src/red.mjs:17` | 2 | 2.6 (M5) | HIGH: SSRF filter bypass for any logged-in user |
| 5 | Owner check splits the raw path on `/`, so `tomas/<own key>/../../…` passes | `motor/src/lab.mjs:291-299` | 1 | 1.7 | HIGH: a logged-in creator reads other people's raw takes |
| 6 | Login fails open: with no https public URL, Google or SMTP configured, every caller is a superadmin | `motor/src/acceso.mjs:39,312` | 1 | 1.1 (M1) | CRITICAL: anyone on the internet gets every right, one unset variable away |
| 7 | `modelo` passed unescaped through `cmd.exe` when the connector's Claude is a `.cmd` | `motor/src/claude-local.mjs:103,114` | 2 | 2.1 | MEDIUM: the value comes from the server, so the exploit needs the server already taken; the payoff is a command on each connected colleague's computer |
| 8 | Daily backup dumps every table as plaintext `.json.gz` into `.lab/respaldos/`; the tables hold scrypt hashes and token hashes, not the secrets, and beside them personal data: the team roster with emails and roles, Claude usage per person, and Manychat's log of Instagram commenters' usernames and comments | `motor/src/db.mjs:338-341,394-396`, `motor/src/db/esquema.mjs:50-52,59-63,69-73,80-82,108`; roster: `equipo` entries in `docs` (`motor/src/db/esquema.mjs:18-25`, `motor/src/acceso.mjs:230`); usage per person: `uso` and its `quien` column (`motor/src/db/esquema.mjs:28-40`); `motor/src/manychat.mjs:112,169`, `DESPLIEGUE.md:25`; written at `motor/src/lab.mjs:54,1598,1660` | 3 | 3.4 | MEDIUM, by axis 3's rule on copies and the rubric's "needs the server already compromised", whatever it holds: no route serves the files or their folder — `/medio` refuses dot-folders and any `.gz` (`motor/src/lab.mjs:116-129,1606-1609`), `/foto/` serves only profile photos by a fixed pattern (`:1161-1162`), `/api/respaldo` returns the file's path, not its contents (`:1596-1599`), row 5's traversal hands a file to the video motor, not back to the caller (`:797-799,1415`), and Caddy only proxies to the app (`DESPLIEGUE.md:281-289`); row 1's file read reaches them, so by the Severity rule on a sink only another confirmed finding reaches, they are rated in row 1 |
| 9 | Credentials committed to `trii-co` repositories: an active service-account key with admin roles, a hardcoded Django `SECRET_KEY` on a public service, mTLS and FIX private keys | `diagnostico-2026-09-18.md:79-95` | 3 | 3.1 (M3) | CRITICAL: live, and opens production systems |
| 10 | `be-webhooks` public receiver ingests financial notifications with no token, HMAC or IP allowlist | `diagnostico-2026-09-18.md:120-126` | 1 | 1.5 | CRITICAL: anyone injects broker notifications |
| 11 | End-user auth is a per-handler `sk2` check, with no central control and no inventory of routes without it | `diagnostico-2026-09-18.md:139-142` | 1 | 1.4 | MEDIUM: the pattern; each route it misses is its own finding |
| 12 | Workflows authenticate to GCP with static JSON keys (`credentials_json`) and pin no third-party action by SHA | `diagnostico-2026-09-18.md:145-154`, esp. `:149-150` | 5 | 5.4, 5.3 | HIGH: a moved tag runs code holding a production key |
| 13 | Dockerfiles run as root, on base images with no digest and `alpine:latest` | `diagnostico-2026-09-18.md:182-188`, esp. `:184,187` | 4, 5 | 4.1 (M4), 5.5 | MEDIUM: amplifies any execution finding; M4 absent fails the bar on its own |

Against the bar, lab-trii-contenido at `98d78fd` is **not fit to expose**: one
Critical, five Highs, and M1, M2 and M5 absent — its Google login compares `hd` only
when present and sends no nonce (`motor/src/acceso.mjs:191-195,219`).
