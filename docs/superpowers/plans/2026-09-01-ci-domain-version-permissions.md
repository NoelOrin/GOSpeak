# CI 分域、版本同步与权限最小化 实现计划

> **面向 AI 代理的工作者：** 本计划在当前工作区内执行，步骤使用复选框跟踪进度。

**目标：** 将 GOSpeak 的 CI 整理为稳定质量门禁、独立安全检查、独立镜像发布和受限权限的发布链路，并统一产品版本校验。

**架构：** `CI` 保留为 Pull Request 的总门禁，质量、测试、构建和版本检查通过稳定的 `gate` 汇总；安全、文档、Docker、Release Please 和产物发布分别按职责运行。产品版本由根目录 `VERSION` 管理，Release Please 同步根 `package.json` 和 manifest，发布 Tag 必须与版本一致。

**技术栈：** GitHub Actions、Node.js 原生测试、pnpm、Go、Docker、Release Please。

---

## 文件结构

| 文件 | 职责 |
|---|---|
| `VERSION` | 产品版本唯一来源 |
| `scripts/check-version.mjs` | 读取并校验版本文件、根 package、Release Please manifest 和 Tag |
| `scripts/check-version.test.mjs` | 版本校验行为测试 |
| `package.json` | 暴露 `version:check`，同步根版本 |
| `release-please-config.json` | 配置 Release Please 同步版本文件和根 package |
| `.github/workflows/ci.yml` | 质量、测试、构建、版本检查和总门禁 |
| `.github/workflows/security.yml` | Go 和 pnpm 依赖安全检查 |
| `.github/workflows/docker.yml` | Pull Request 构建、Tag 正式推送 |
| `.github/workflows/release.yml` | 二进制和校验和发布，不再负责 Docker |
| `.github/workflows/release-please.yml` | 仅授予 Release Please 所需权限 |
| `.github/workflows/docs.yml` | 仅授予 Pages 部署和重新触发所需权限 |
| `.github/workflows/build.yml` | 删除废弃兼容层 |

## 实施步骤

- [ ] 为版本检查编写失败测试并验证失败。
- [ ] 实现版本检查脚本，补齐 `VERSION`、package script 和 Release Please 配置。
- [ ] 为 CI 增加稳定 `gate`，补充 JavaScript 测试和版本检查。
- [ ] 新增独立安全和 Docker 工作流，移除 Release 中的 Docker Job。
- [ ] 收紧所有发布相关权限并删除废弃工作流。
- [ ] 运行脚本、YAML、前端、Go 和发布配置验证。
