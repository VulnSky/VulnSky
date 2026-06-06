# 阿里云权限参考

建议为 VulnSky 创建单独 RAM 用户，按最小权限授权，并定期轮换 AccessKey。

## API 能力

VulnSky 当前会用到以下能力：

| 服务 | 用途 | 相关动作 |
| --- | --- | --- |
| STS | 检查 AccessKey 所属账号 | `sts:GetCallerIdentity` |
| OSS | 列目录、检查对象、上传 QCOW2、生成下载链接、查询 bucket 区域 | `oss:ListObjects`、`oss:GetObject`、`oss:PutObject`、`oss:GetBucketInfo` |
| ECS | 查询 ECS 和镜像 | `ecs:DescribeInstances`、`ecs:DescribeImages`、`ecs:DescribeTasks` |
| ECS | 导入自定义镜像 | `ecs:ImportImage` |
| ECS | 靶机重装流程 | `ecs:StopInstance`、`ecs:StartInstance`、`ecs:ReplaceSystemDisk` |

## 授权建议

- OSS 权限尽量限制到指定 bucket 和 `qcow2/` 前缀。
- ECS 权限尽量限制到教学靶机实例和自定义镜像相关资源。
- 不要给生产 ECS 使用同一组 AccessKey。
- 如果使用 `ImportImage` 需要 RAM 角色，请按阿里云控制台提示配置 `--role-name` 或默认导入角色。

## 发布前自检

配置好 AccessKey 后运行：

```powershell
.\dist\vulnsky.exe doctor
```

`doctor` 会检查：

- `.env` 和当前 profile 是否存在。
- STS 调用是否有效。
- OSS bucket 区域是否与配置一致。
- ECS 区域是否可访问。
- 默认 ECS 是否存在。
