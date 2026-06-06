## Summary

Describe the change briefly.

## Verification

- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `scripts/verify-release.ps1` or `scripts/verify-release.sh`

## Safety

- [ ] No `.env`, profile env, database, log, or image files are included.
- [ ] Cloud-mutating behavior is covered by tests or documented manual verification.
