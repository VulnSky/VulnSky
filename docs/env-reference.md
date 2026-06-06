# 环境变量参考

VulnSky 使用根目录 `.env` 保存全局设置，使用 `profiles/<name>.env` 保存阿里云账号、区域和默认 ECS。

## `.env`

| 变量 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `VULNSKY_ACTIVE_PROFILE` | 否 | `default` | 当前默认 profile。 |
| `VULNSKY_DB_PATH` | 否 | `./vulnsky.db` | 本地 SQLite 数据库路径。 |
| `VULNSKY_PROFILE_DIR` | 否 | `./profiles` | profile 文件目录。 |

## `profiles/<name>.env`

| 变量 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `VULNSKY_PROFILE_LABEL` | 否 | profile 名称 | 人类可读标签。 |
| `VULNSKY_EXPECTED_ACCOUNT_ID` | 否 | 空 | `doctor` 用于校验 AccessKey 是否属于预期账号。 |
| `ALIBABA_CLOUD_ACCESS_KEY_ID` | 是 | 空 | ECS/STS 使用的 AccessKey ID。 |
| `ALIBABA_CLOUD_ACCESS_KEY_SECRET` | 是 | 空 | ECS/STS 使用的 AccessKey Secret。 |
| `ALIBABA_CLOUD_REGION_ID` | 是 | 空 | ECS 区域，例如 `cn-beijing`。 |
| `ALIBABA_OSS_ACCESS_KEY_ID` | 否 | `ALIBABA_CLOUD_ACCESS_KEY_ID` | OSS 单独 AccessKey ID。 |
| `ALIBABA_OSS_ACCESS_KEY_SECRET` | 否 | `ALIBABA_CLOUD_ACCESS_KEY_SECRET` | OSS 单独 AccessKey Secret。 |
| `ALIBABA_OSS_REGION_ID` | 是 | `ALIBABA_CLOUD_REGION_ID` | OSS bucket 区域。导入镜像时必须与 ECS 区域一致。 |
| `ALIBABA_OSS_ENDPOINT` | 是 | 空 | OSS endpoint，例如 `https://oss-cn-beijing.aliyuncs.com`。 |
| `ALIBABA_OSS_BUCKET` | 是 | 空 | 保存 QCOW2 的 bucket。 |
| `VULNSKY_DEFAULT_ECS_INSTANCE_ID` | 建议 | 空 | 默认重装目标 ECS。 |
| `VULNSKY_DEFAULT_OBJECT_PREFIX` | 否 | `qcow2/` | 本地 QCOW2 上传到 OSS 时的默认前缀。 |
| `VULNSKY_DEFAULT_ARCHITECTURE` | 否 | `x86_64` | `ImportImage` 的架构。 |
| `VULNSKY_DEFAULT_OS_TYPE` | 否 | `linux` | `ImportImage` 的 OS 类型。 |
| `VULNSKY_DEFAULT_PLATFORM` | 否 | `Others Linux` | `ImportImage` 的平台类型。 |
| `VULNSKY_AUTO_STOP_INSTANCE` | 否 | `true` | 预留配置，目前重装流程会按命令执行停机。 |
| `VULNSKY_ALLOW_FORCE_STOP` | 否 | `false` | 是否默认允许强制关机。 |
| `VULNSKY_STOP_TIMEOUT_SECONDS` | 否 | `60` | 等待 ECS 停机的默认超时时间。 |
| `VULNSKY_START_AFTER_REIMAGE` | 否 | `true` | 替换系统盘后是否自动启动 ECS。 |

## 最小可用配置

```env
ALIBABA_CLOUD_ACCESS_KEY_ID=
ALIBABA_CLOUD_ACCESS_KEY_SECRET=
ALIBABA_CLOUD_REGION_ID=cn-beijing
ALIBABA_OSS_REGION_ID=cn-beijing
ALIBABA_OSS_ENDPOINT=https://oss-cn-beijing.aliyuncs.com
ALIBABA_OSS_BUCKET=
VULNSKY_DEFAULT_ECS_INSTANCE_ID=
```
