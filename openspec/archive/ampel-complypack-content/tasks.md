## 1. Secure Tar Extraction

- [x] 1.1 Create `cmd/ampel-provider/server/unpack.go` with `resolveComplypackPath()` function that handles directory pass-through and tar.gz archive extraction with idempotent sibling `content/` directory reuse
- [x] 1.2 Add `extractTarGz()` function with zip-slip protection (reject `../` and absolute paths), symlink/hard-link rejection, and 100 MB per-file size cap
- [x] 1.3 Add `writeFileFromTar()` helper that writes files with mode `0600` and caps writes at `maxExtractedFileSize`
- [x] 1.4 Create `cmd/ampel-provider/server/unpack_test.go` with tests: valid directory pass-through, valid tar.gz extraction, idempotent re-extraction skip, path traversal rejection, symlink rejection, oversized file rejection, non-existent path error, corrupt archive error

## 2. Generate Integration

- [x] 2.1 Update `Generate()` in `cmd/ampel-provider/server/server.go` to check `req.ComplypackContentPath` before `ampel_policy_dir` and default path, using the precedence order: ComplypackContentPath > ampel_policy_dir > default
- [x] 2.2 Add required imports to `server.go` (`archive/tar`, `compress/gzip`, `io`, `os`, `path/filepath`, `strings` -- as needed for the unpack file)
- [x] 2.3 Add integration tests in `cmd/ampel-provider/server/server_test.go`: Generate with complypack directory path, Generate with complypack tar.gz path, complypack precedence over ampel_policy_dir, backward compatibility when ComplypackContentPath is empty

## 3. Verification

- [x] 3.1 Run `make test` and confirm all existing and new tests pass
- [x] 3.2 Run `make lint` and fix any linting issues
- [x] 3.3 Run `make build` and confirm the ampel provider binary builds successfully
