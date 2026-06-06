# VulnSky

VulnSky 是一个面向教学靶机部署的 CLI 工具。它可以把本地 QCOW2 上传到阿里云 OSS，调用 ECS `ImportImage` 生成自定义镜像，再把指定 ECS 的系统盘替换为该镜像。上传、镜像导入、ECS 重装等操作会记录到本地 SQLite 数据库，方便后续查询和复用。

适用场景：教师或实验环境维护者把靶机部署到公网 ECS，并自行通过安全组、防火墙、IP 白名单等方式限制学生访问范围。

## 功能

- OSS：列目录、上传 QCOW2、生成临时下载链接。
- Image：从 OSS 对象导入 ECS 自定义镜像、查询导入任务、查看镜像来源 QCOW2。
- ECS：查询实例、设置默认实例、查看当前镜像、启动实例、用已有镜像重装系统盘。
- Deploy：一条命令完成本地 QCOW2 上传、镜像导入、替换默认 ECS 系统盘。
- Records：查询本地上传、镜像导入、部署历史。
- Profile：支持多个 profile，对应不同阿里云账号或 AccessKey。
- Shell：支持 `vulnsky/> oss`、`vulnsky/oss> ls` 这种交互式上下文命令。
- Version：二进制可输出版本、commit 和构建时间，方便定位 Release 版本。

## 安装与构建

普通使用建议从 GitHub Release 下载对应系统的归档包：

- Windows 10/11：`vulnsky-windows-amd64.zip` 或 `vulnsky-windows-arm64.zip`
- Debian/Ubuntu/Fedora：`vulnsky-linux-amd64.tar.gz` 或 `vulnsky-linux-arm64.tar.gz`
- macOS：`vulnsky-darwin-amd64.tar.gz` 或 `vulnsky-darwin-arm64.tar.gz`

下载后可用 Release 附带的 `SHA256SUMS` 校验归档包。

详细安装、校验和 PATH 配置见 [docs/install.md](docs/install.md)。

从源码构建需要 Go 1.26.x。

```powershell
go build -trimpath -buildvcs=false -ldflags "-s -w" -o dist\vulnsky.exe .\cmd\vulnsky
```

Linux/macOS:

```bash
go build -trimpath -buildvcs=false -ldflags "-s -w" -o dist/vulnsky ./cmd/vulnsky
```

本地多平台构建：

```powershell
.\scripts\build-release.ps1
```

如果 Go 不在 PATH，可以先设置 `$env:GO_EXE`。

Linux/macOS:

```bash
./scripts/build-release.sh
```

Linux/macOS 也可以通过 `GO_EXE=/path/to/go` 指定 Go。

如果仓库已经发布到 GitHub，并希望支持 `go install github.com/<owner>/vulnsky/cmd/vulnsky@latest`，请先把 Go module path 改成正式仓库路径：

```powershell
.\scripts\set-module-path.ps1 github.com/<owner>/vulnsky
```

Linux/macOS:

```bash
./scripts/set-module-path.sh github.com/<owner>/vulnsky
```

## 初始化配置

首次运行真实子命令时，如果当前目录没有 `.env` 和 `profiles/default.env`，VulnSky 会自动创建默认模板。也可以手动复制示例：

```powershell
Copy-Item .env.example .env
Copy-Item profiles\default.env.example profiles\default.env
```

常用必填项：

- `ALIBABA_CLOUD_ACCESS_KEY_ID`
- `ALIBABA_CLOUD_ACCESS_KEY_SECRET`
- `ALIBABA_CLOUD_REGION_ID`
- `ALIBABA_OSS_REGION_ID`
- `ALIBABA_OSS_ENDPOINT`
- `ALIBABA_OSS_BUCKET`
- `VULNSKY_DEFAULT_ECS_INSTANCE_ID`

如果 OSS 和 ECS 使用同一组 AccessKey，可以不填 `ALIBABA_OSS_ACCESS_KEY_ID` 和 `ALIBABA_OSS_ACCESS_KEY_SECRET`，工具会复用 `ALIBABA_CLOUD_*`。

注意：`ImportImage` 要求 OSS bucket 与 ECS 镜像导入区域一致，例如都在 `cn-beijing`。

完整配置项见 [docs/env-reference.md](docs/env-reference.md)，阿里云 RAM 权限参考见 [docs/aliyun-permissions.md](docs/aliyun-permissions.md)，常见问题见 [docs/troubleshooting.md](docs/troubleshooting.md)。

## 常用命令

检查配置和云 API：

```powershell
.\dist\vulnsky.exe doctor
.\dist\vulnsky.exe doctor --redact
```

管理 profile：

```powershell
.\dist\vulnsky.exe profile ls
.\dist\vulnsky.exe profile use default
.\dist\vulnsky.exe profile show default
```

OSS：

```powershell
.\dist\vulnsky.exe oss ls qcow2/
.\dist\vulnsky.exe oss upload "C:\Labs\sample-lab.qcow2" --key qcow2/sample-lab.qcow2
.\dist\vulnsky.exe oss link qcow2/sample-lab.qcow2 --expires 6h
```

镜像：

```powershell
.\dist\vulnsky.exe image import qcow2/sample-lab.qcow2 --name vulnsky-sample-lab
.\dist\vulnsky.exe image status t-xxxxxxxx
.\dist\vulnsky.exe image ls
.\dist\vulnsky.exe image source m-xxxxxxxx
```

ECS：

```powershell
.\dist\vulnsky.exe ecs ls
.\dist\vulnsky.exe ecs use i-xxxxxxxx
.\dist\vulnsky.exe ecs current-image
.\dist\vulnsky.exe ecs reimage m-xxxxxxxx --force-stop
```

一键部署本地 QCOW2 到默认 ECS：

```powershell
.\dist\vulnsky.exe deploy "C:\Labs\sample-lab.qcow2" --key qcow2/sample-lab.qcow2 --force-stop
```

使用已有镜像直接重装默认 ECS：

```powershell
.\dist\vulnsky.exe ecs reimage m-xxxxxxxx --force-stop
.\dist\vulnsky.exe deploy --image-id m-xxxxxxxx --force-stop
```

查看本地记录：

```powershell
.\dist\vulnsky.exe records uploads
.\dist\vulnsky.exe records images
.\dist\vulnsky.exe records deployments
```

查看版本：

```powershell
.\dist\vulnsky.exe version
```

生成 shell 补全脚本：

```powershell
.\dist\vulnsky.exe completion powershell
```

Linux/macOS:

```bash
vulnsky completion bash
vulnsky completion zsh
vulnsky completion fish
```

## 交互式 shell

直接运行 `vulnsky` 或 `vulnsky shell` 会进入交互模式：

```text
vulnsky/> help
vulnsky/> oss
vulnsky/oss> ls qcow2/
vulnsky/oss> back
vulnsky/> ecs
vulnsky/ecs> current-image
vulnsky/ecs> exit
```

在上下文里也可以直接运行根命令，例如在 `vulnsky/oss>` 中输入 `ecs ls` 会执行根级 `ecs ls`。

## 发布

仓库内置 GitHub Actions：

- `.github/workflows/ci.yml`：push 和 pull request 时运行测试、vet、构建。
- `.github/workflows/release.yml`：推送 `v*` 标签时构建 Windows、Linux、macOS 的 amd64/arm64 二进制并创建 GitHub Release。
- Release 产物会注入版本信息，Windows 使用 `.zip`，Linux/macOS 使用 `.tar.gz`，并生成 `SHA256SUMS`。
- `.github/dependabot.yml`：定期检查 Go module 和 GitHub Actions 依赖更新。

本地发布前可运行：

```powershell
.\scripts\verify-release.ps1
```

更多步骤见 [docs/github-release-checklist.md](docs/github-release-checklist.md)。

## 安全提示

- 不要提交 `.env`、`profiles/*.env`、`vulnsky.db`、日志和 `dist/` 产物。
- AccessKey 建议使用最小权限 RAM 用户，并定期轮换。
- 提交 issue 或分享诊断日志时，优先使用 `vulnsky doctor --redact`。
- 重装 ECS 会替换系统盘，请确认目标实例是教学靶机，不要对生产实例运行。
- 公网访问控制不由 VulnSky 负责，需要在阿里云安全组、防火墙、白名单等位置单独配置。

## 依赖

本项目使用 Go 和阿里云官方 SDK：

- `github.com/alibabacloud-go/ecs-20140526/v4`
- `github.com/alibabacloud-go/sts-20150401/v2`
- `github.com/aliyun/alibabacloud-oss-go-sdk-v2`
