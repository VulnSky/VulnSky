# 贡献指南

VulnSky 目前主要面向个人教学环境使用，公开仓库优先接受小范围、可验证的改动。

## 开发环境

- Go 1.26.x
- Windows 10/11 或 Linux/macOS
- 可选：WSL Ubuntu 24.04

## 本地验证

提交前至少运行：

```powershell
go test ./...
go vet ./...
go build -o dist\vulnsky.exe .\cmd\vulnsky
```

也可以运行：

```powershell
.\scripts\verify-release.ps1
```

如果 Go 不在 PATH，可以设置：

```powershell
$env:GO_EXE="C:\Path\To\go.exe"
.\scripts\verify-release.ps1
```

Linux/macOS:

```bash
./scripts/verify-release.sh
```

## 配置与密钥

不要提交以下文件：

- `.env`
- `.env.*`
- `profiles/*.env`
- `vulnsky.db`
- `dist/`
- 日志文件

如果需要新增配置项，请同步更新：

- `.env.example`
- `profiles/default.env.example`
- `README.md`

如果发布仓库需要支持 `go install`，使用 `scripts/set-module-path.ps1` 或 `scripts/set-module-path.sh` 更新 module path，不要只手改 `go.mod`。

## 代码约定

- 业务逻辑优先复用现有 command/config/store/aliyun 分层。
- 新增命令需要补测试。
- 涉及云资源变更的命令必须输出阶段性日志，并尽量写入本地记录。
