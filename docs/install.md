# 安装与校验

VulnSky 可以直接下载 GitHub Release 里的二进制归档包使用，不要求目标机器预装 Go。建议为 VulnSky 准备一个固定工作目录；默认情况下 `.env`、`profiles/` 和 `vulnsky.db` 会创建在运行命令的当前目录。

如果希望在其他目录运行命令，可以用 `--root <work-dir>` 指定 VulnSky 工作目录。

## 选择归档包

| 系统 | amd64 | arm64 |
| --- | --- | --- |
| Windows 10/11 | `vulnsky-windows-amd64.zip` | `vulnsky-windows-arm64.zip` |
| Debian/Ubuntu/Fedora | `vulnsky-linux-amd64.tar.gz` | `vulnsky-linux-arm64.tar.gz` |
| macOS | `vulnsky-darwin-amd64.tar.gz` | `vulnsky-darwin-arm64.tar.gz` |

## 校验下载

下载归档包后，同时下载 Release 里的 `SHA256SUMS`。

Windows PowerShell:

```powershell
Get-FileHash .\vulnsky-windows-amd64.zip -Algorithm SHA256
Get-Content .\SHA256SUMS
```

Linux:

```bash
sha256sum -c SHA256SUMS --ignore-missing
```

macOS:

```bash
shasum -a 256 vulnsky-darwin-arm64.tar.gz
cat SHA256SUMS
```

## Windows

```powershell
New-Item -ItemType Directory -Force "$HOME\vulnsky\bin"
Expand-Archive .\vulnsky-windows-amd64.zip "$HOME\vulnsky\bin" -Force
New-Item -ItemType Directory -Force "$HOME\vulnsky\work"
Set-Location "$HOME\vulnsky\work"
..\bin\vulnsky-windows-amd64.exe version
..\bin\vulnsky-windows-amd64.exe doctor --redact
```

也可以把 `vulnsky-windows-amd64.exe` 重命名为 `vulnsky.exe`，并把 `$HOME\vulnsky\bin` 加入 PATH。

## Linux

```bash
mkdir -p "$HOME/.local/bin" "$HOME/vulnsky-work"
tar -xzf vulnsky-linux-amd64.tar.gz -C "$HOME/.local/bin"
mv "$HOME/.local/bin/vulnsky-linux-amd64" "$HOME/.local/bin/vulnsky"
chmod +x "$HOME/.local/bin/vulnsky"
cd "$HOME/vulnsky-work"
vulnsky version
vulnsky doctor --redact
```

如果 `$HOME/.local/bin` 不在 PATH，请把它加入 shell 配置。

## macOS

```bash
mkdir -p "$HOME/.local/bin" "$HOME/vulnsky-work"
tar -xzf vulnsky-darwin-arm64.tar.gz -C "$HOME/.local/bin"
mv "$HOME/.local/bin/vulnsky-darwin-arm64" "$HOME/.local/bin/vulnsky"
chmod +x "$HOME/.local/bin/vulnsky"
cd "$HOME/vulnsky-work"
vulnsky version
vulnsky doctor --redact
```

如果 macOS 阻止运行未签名二进制，请在系统安全设置里允许该二进制，或从源码自行构建。

## 初始化配置

首次运行真实子命令时，如果当前目录没有 `.env` 和 `profiles/default.env`，VulnSky 会自动创建默认模板。配置 AccessKey、区域、OSS bucket 和默认 ECS 后，运行：

```bash
vulnsky doctor --redact
```

`doctor --redact` 会检查云 API 可用性，同时隐藏本地路径、账号、ARN 和 ECS 标识，适合用于公开 issue 或截图。
