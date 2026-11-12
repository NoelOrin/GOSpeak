# Test Logging Skill

## 描述

当 agent 被命令进行测试时，必须将测试总结的结果以 Markdown 格式保存到 `agent_test_logs` 文件夹中。无特定要求时,报告为中文.

## 触发条件

- 用户要求测试 API
- 用户要求测试功能
- 用户要求验证系统行为
- 用户要求执行测试用例

## 执行流程

### 1. 准备阶段

1. 确认测试目标和范围
2. 确保服务器已启动
3. 准备测试数据和 token

### 2. 任务拆分与并发执行

当测试项数量 >= 3 时，应使用 Agent 工具拆分为多个 subagent 并发执行：

**拆分策略：**
1. 按 API 模块或功能域分组（如认证模块、用户模块、房间模块）
2. 每个 subagent 负责一组测试项，共享同一份 token 和上下文
3. 使用单条消息中的多个 Agent 工具调用实现真正并发

**上下文共享：**
- 将登录获取的 token、测试用户名密码等公共数据作为 prompt 的一部分传递给每个 subagent
- 每个 subagent 的 prompt 必须自包含，不依赖其他 subagent 的输出
- 最终由主 agent 汇总所有 subagent 的测试结果

**示例拆分：**
```
# 主 agent 同时发出多个 Agent 调用
Agent(description="测试认证API", prompt="使用 token xxx 测试以下接口：登录/登出/刷新token...")
Agent(description="测试用户API", prompt="使用 token xxx 测试以下接口：用户列表/个人资料/修改角色...")
Agent(description="测试房间API", prompt="使用 token xxx 测试以下接口：创建房间/房间列表...")
```

**汇总规则：**
1. 收集所有 subagent 返回的测试结果
2. 合并为统一的测试报告
3. 按原始测试计划排序

### 3. 执行测试（顺序模式）

当测试项 < 3 或无法并行时，按以下流程执行：
1. 按照测试计划逐项执行
2. 记录每个测试项的请求和响应
3. 验证测试结果是否符合预期
4. 不要删除数据库

### 4. 生成报告

1. 使用统一的报告模板
2. 填写测试概要和结果汇总
3. 记录详细测试过程
4. 总结问题和建议

### 5. 保存报告

1. 按照命名规范生成文件名
2. 将报告保存到 `agent_test_logs/` 目录
3. 确认文件已成功创建

## 命名规范

文件名格式：`{测试内容}-{时间}.md`

### 示例

- `api-auth-test-2026-05-26.md`
- `role-permission-test-2026-05-26.md`
- `user-crud-test-2026-05-26-14-30.md`

### 规则

1. 使用小写字母和连字符（kebab-case）
2. 测试内容简明扼要，体现测试范围
3. 时间格式：`YYYY-MM-DD` 或 `YYYY-MM-DD-HH-MM`
4. 必须以 `.md` 结尾

## 报告模板

```markdown
# {测试标题}

**测试时间**: YYYY-MM-DD HH:MM:SS
**测试环境**: dev / prod
**测试模型**: deepseek-v4-flash

## 测试概要

简要描述本次测试的目的和范围。

## 测试结果

| 测试项 | 状态 | 说明 |
|--------|------|------|
| 测试项1 | ✅ 通过 | 成功说明 |
| 测试项2 | ❌ 失败 | 失败原因 |

## 详细测试记录

### 1. 测试项名称

**请求**:
```bash
curl -X POST http://localhost:8998/api/v1/xxx
```

**响应**:
```json
{
  "code": 0,
  "msg": "success",
  "data": { ... }
}
```

**结论**: 测试通过/失败，原因说明。

## 问题与建议

列出测试过程中发现的问题和改进建议。

## 总结

整体测试结论和下一步计划。
```

## 测试状态标识

- ✅ 通过 - 测试成功
- ❌ 失败 - 测试失败
- ⚠️ 警告 - 测试通过但存在问题
- ⏭️ 跳过 - 测试被跳过

## 注意事项

1. 测试前确保服务器已启动
2. 测试数据应使用测试专用数据，避免影响生产数据
3. 敏感信息（如 token）在报告中应适当截断
4. 测试报告应及时生成，避免遗忘测试细节
5. 失败的测试项必须记录失败原因和错误信息

## 示例

### 测试认证 API

```bash
# 1. 注册用户
curl -X POST http://localhost:8998/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"123456"}'

# 2. 登录获取 token
curl -X POST http://localhost:8998/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"123456"}'

# 3. 使用 token 访问受保护接口
curl -X GET http://localhost:8998/api/v1/user/profile \
  -H "Authorization: Bearer $TOKEN"
```

### 测试权限控制

```bash
# 1. 普通用户尝试访问管理员接口
curl -X DELETE http://localhost:8998/api/v1/user/1 \
  -H "Authorization: Bearer $USER_TOKEN"

# 预期响应：403 Forbidden
# {
#   "code": 1013,
#   "msg": "forbidden",
#   "data": null
# }
```
