# 排障指南

## 先运行 doctor

大多数配置问题先用 `doctor` 定位：

```powershell
.\dist\vulnsky.exe doctor
.\dist\vulnsky.exe doctor --redact
```

提交 issue 或分享日志时优先使用 `--redact`，避免泄露本地路径、账号、ARN 和 ECS 标识。

## `.env` 或 profile 未找到

VulnSky 默认在当前运行目录查找 `.env` 和 `profiles/default.env`。建议先进入固定工作目录再运行命令：

```powershell
New-Item -ItemType Directory -Force "$HOME\vulnsky\work"
Set-Location "$HOME\vulnsky\work"
vulnsky doctor --redact
```

也可以显式指定工作目录：

```powershell
vulnsky --root "$HOME\vulnsky\work" doctor --redact
```

首次运行真实子命令时会自动生成模板文件，也可以手动复制仓库里的 `.env.example` 和 `profiles/default.env.example`。

## AccessKey 或账号不匹配

如果 `doctor` 的 STS 检查失败，优先检查：

- `ALIBABA_CLOUD_ACCESS_KEY_ID`
- `ALIBABA_CLOUD_ACCESS_KEY_SECRET`
- `ALIBABA_CLOUD_REGION_ID`
- `VULNSKY_EXPECTED_ACCOUNT_ID`

如果 OSS 使用同一组 AccessKey，可以不填 `ALIBABA_OSS_ACCESS_KEY_ID` 和 `ALIBABA_OSS_ACCESS_KEY_SECRET`。

## ECS/OSS 区域不一致

`ImportImage` 要求 OSS bucket 与 ECS 镜像导入区域一致。例如 ECS 在 `cn-beijing`，OSS bucket 也应在 `cn-beijing`。

需要同步检查：

- `ALIBABA_CLOUD_REGION_ID`
- `ALIBABA_OSS_REGION_ID`
- `ALIBABA_OSS_ENDPOINT`
- OSS bucket 实际区域

## 默认 ECS 未配置

先查询实例，再设置默认 ECS：

```powershell
.\dist\vulnsky.exe ecs ls
.\dist\vulnsky.exe ecs use i-xxxxxxxx
.\dist\vulnsky.exe ecs current-image
```

之后 `deploy` 或 `ecs reimage` 不传 `--instance-id` 时会使用默认 ECS。

## 镜像导入长时间 Waiting 或 Processing

`ImportImage` 是异步任务，等待时间与 QCOW2 大小、区域资源和阿里云导入队列有关。可以单独查询任务：

```powershell
.\dist\vulnsky.exe image status t-xxxxxxxx
```

如果任务失败，检查 QCOW2 文件格式、OSS 对象路径、RAM 权限和 ECS 镜像导入角色。

## ECS 启动或停机状态不接受操作

重装系统盘前实例需要进入可操作状态。`deploy` 和 `ecs reimage` 会轮询停机和启动过程；如果云 API 返回 `IncorrectInstanceStatus`，通常是实例状态还在转换中。

可以稍等后查询：

```powershell
.\dist\vulnsky.exe ecs show i-xxxxxxxx
.\dist\vulnsky.exe ecs start i-xxxxxxxx
```

如果长期停留在异常状态，请到阿里云 ECS 控制台检查实例事件和磁盘状态。

## 本地记录查询

VulnSky 会把上传、镜像导入和部署记录保存在本地 SQLite 数据库：

```powershell
.\dist\vulnsky.exe records uploads
.\dist\vulnsky.exe records images
.\dist\vulnsky.exe records deployments
```

如果在不同目录运行，会使用不同的 `vulnsky.db`。需要共享记录时，请在 `.env` 里设置固定的 `VULNSKY_DB_PATH`。
