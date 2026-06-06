# GitHub 发布检查清单

## 发布前

- [ ] 确认 `.env`、`profiles/*.env`、`vulnsky.db`、日志和 `dist/` 没有进入 Git。
- [ ] 运行 `go test ./...`。
- [ ] 运行 `go vet ./...`。
- [ ] 运行 `go build -trimpath -buildvcs=false -ldflags "-s -w" -o dist\vulnsky.exe .\cmd\vulnsky`。
- [ ] 运行 `.\dist\vulnsky.exe --help`。
- [ ] 运行 `.\dist\vulnsky.exe version`。
- [ ] 运行 `.\dist\vulnsky.exe doctor` 检查本机配置。
- [ ] 运行 `.\dist\vulnsky.exe doctor --redact` 检查公开日志不会泄露本地路径和云资源标识。
- [ ] 检查 README 中的命令示例与当前 CLI 一致。
- [ ] 检查 [install.md](install.md) 中的归档包名称与 Release 产物一致。

## 初次发布

如果需要支持 `go install github.com/<owner>/vulnsky/cmd/vulnsky@latest`，先更新 module path：

```powershell
.\scripts\set-module-path.ps1 github.com/<owner>/vulnsky
git add .
git commit --amend --no-edit
```

```powershell
git init
git add .
git status --ignored
git commit -m "Initial public release"
git branch -M main
git remote add origin https://github.com/<owner>/vulnsky.git
git push -u origin main
```

## 打标签发布二进制

```powershell
git tag v0.1.0
git push origin v0.1.0
```

推送 `v*` 标签后，GitHub Actions 会构建 Windows、Linux、macOS 的 amd64/arm64 产物并创建 Release。
Release 附件会包含 `vulnsky-windows-amd64.zip`、`vulnsky-linux-amd64.tar.gz` 等多平台归档包和 `SHA256SUMS`，用于校验下载的归档包。
