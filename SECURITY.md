# 安全说明

VulnSky 会操作阿里云 OSS、ECS 镜像和 ECS 系统盘。请只在授权的教学或实验环境中使用。

## 密钥管理

- 使用 RAM 子账号和最小权限策略。
- 不要提交 `.env`、`profiles/*.env` 或任何包含 AccessKey 的文件。
- 定期轮换 AccessKey。
- 公开仓库前建议启用 GitHub Secret Scanning。

## 云资源风险

- `deploy` 和 `ecs reimage` 会替换 ECS 系统盘。
- 目标 ECS 应为专用靶机，不要指向生产实例。
- VulnSky 不负责公网访问控制，请自行配置安全组、防火墙和 IP 白名单。

## 报告安全问题

如果发现密钥泄露、越权调用、错误重装实例等安全问题，请不要公开提交 PoC。建议通过仓库维护者指定的私有渠道报告。
