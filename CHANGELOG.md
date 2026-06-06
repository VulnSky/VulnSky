# Changelog

## 0.1.0

- 支持 profile 配置。
- 支持 OSS 列目录、上传 QCOW2、生成临时下载链接。
- 支持从 OSS QCOW2 导入 ECS 自定义镜像。
- 支持镜像导入任务查询和镜像来源查询。
- 支持查询 ECS、设置默认 ECS、启动 ECS。
- 支持一键部署本地 QCOW2 到默认 ECS。
- 支持用已有镜像直接重装 ECS。
- 支持本地 SQLite 操作记录查询。
- 支持交互式 shell：`vulnsky/> oss`、`vulnsky/oss> ls`。
- 支持 `version` 命令输出版本、commit 和构建时间。
- Release 构建生成多平台归档包和 `SHA256SUMS`，Windows 归档使用 `vulnsky-windows-*.zip` 命名。
- `doctor --redact` 支持输出脱敏诊断信息，方便公开 issue 排障。
- 增加 GitHub Actions、Dependabot、Issue/PR 模板。
- 增加安装与校验文档，覆盖 Windows、Linux、macOS 二进制安装。
- 增加排障文档，覆盖工作目录、区域不一致、导入任务和 ECS 状态问题。
- 增加 `set-module-path` 脚本，方便发布后支持 `go install`。
- 增加环境变量和阿里云权限参考文档。
