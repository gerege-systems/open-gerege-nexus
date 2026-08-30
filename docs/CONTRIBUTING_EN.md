# Contributing

Thank you for your interest in contributing to **Gerege Nexus**
(`open-gerege-nexus`). Community contributions are what make a modular,
high-performance open-source platform possible.

<p>
  <a href="../CONTRIBUTING.md"><img src="assets/icons/flag-mn.png" width="18" height="18" alt=""> Монгол</a>
  &nbsp;·&nbsp;
  <img src="assets/icons/flag-en.png" width="18" height="18" alt=""> <b>English</b>
</p>

---

## Maintainers

- **Gerege Systems Development Team** ([@gerege-systems](https://github.com/gerege-systems))
- **Gemini AI**, **Claude AI**

---

## Code of conduct

Every contributor is expected to follow the
[Code of Conduct](../CODE_OF_CONDUCT.md). Report unacceptable behaviour to
`community@gerege.mn`.

---

## How to contribute

### 1. Reporting bugs

Check the open issues first so you don't file a duplicate. A good report
includes:

- Clear steps to reproduce.
- Your environment (Go version, Node.js version, OS, PostgreSQL version).
- Expected versus actual behaviour, with logs where possible.

### 2. Suggesting enhancements

Describe the use case, the problem you are solving and the solution you have in
mind.

### 3. Submitting pull requests

1. **Create a branch** — `git checkout -b feature/amazing-feature`.
2. **Follow the code conventions**:
   - Backend: Go 1.26+, `gofmt` formatting, structured logging with `slog`,
     explicit error handling.
   - Frontend: Next.js 16 App Router, TypeScript strict mode, Tailwind CSS.
3. **Write tests** — new backend logic ships with `*_test.go` coverage.
4. **Run the verification suite**:

   ```bash
   # Backend: format, static analysis, tests
   cd backend
   gofmt -l .
   go vet ./...
   go test -race ./...
   golangci-lint run

   # Frontend: typecheck and build
   cd ../frontend
   npx tsc --noEmit
   npm run build
   ```

5. **Commit messages** follow
   [Conventional Commits](https://www.conventionalcommits.org/):
   - `feat: add invoice management module`
   - `fix: resolve stock level calculation rounding`
   - `docs: update module authoring guide`
6. **Open the PR** against `main`. Lint, tests, the frontend build and the
   security scans must all be green.

---

## Adding a new business module

1. Create a package under `backend/internal/apps/<module_name>/`.
2. Implement the `internal.Module` interface from `backend/internal/module.go`
   in full.
3. Register the module with `appregistry` and add
   `catalog/manifests/<slug>.json`. The manifest must match
   `appcatalog.Manifest` exactly — a malformed manifest stops the server from
   booting.
4. Add the app to `catalog/apps.json`. The `apps` table is synchronised from
   that file on every boot, so no manual SQL is required.
5. Add the frontend view under `frontend/app/<module_name>/page.tsx`.

[Writing a module](MODULES.md) walks through this in detail.

---

## Documentation and translations

- Mongolian is the primary language. Translations live under `docs/` with the
  `_EN`, `_ZH` and `_RU` suffixes.
- **Do not use emoji in documentation.** Where an icon is needed, use the
  Flaticon assets in [`assets/icons/`](assets/icons/) and add the source to
  [`ATTRIBUTION.md`](assets/icons/ATTRIBUTION.md).
- See [`README.md`](README.md) for how to add a new translation.
