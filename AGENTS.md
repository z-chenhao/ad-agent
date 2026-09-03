# Ad Agent 工程约定

本文件面向开发仓库的 coding agent；`AGENT.md` 是产品广告代理的静态合同。
默认中文沟通。修改前阅读：

- `docs/technical-design-v0.md`
- `docs/ad-backend-contract.md`
- `docs/development-readiness.md`

## 边界

- Go 拥有领域、AdBackend、executor、snapshot、审批、审计和 HTTP/SSE。
- React/TypeScript 拥有页面；Pi TypeScript sidecar 仅负责 runtime/session。
- AdBackend 首版只读，TikTok MAPI 与 fixture 分别实现，runtime 独立替换。
- host change service 独占 writer；模型无 apply 工具，聊天不是审批。
- 身份/环境由 host 绑定；fixture 不回退真实错误，也不冒充 live 数据。
- Pi 显式选择 openai-codex/gpt-5.6-luna，不静默换模型或修改全局默认设置。
- 显式初始化 Pi 网络层，禁用默认编码工具和自动 contexts/extensions/skills/prompts。
- API 凭证不进入模型业务上下文、SSE、日志或 Git；provider transcript 保持私有。
- 3000 仅用于隔离回调，不启动完整管理应用或文件服务器；官方支持 localhost，ngrok
  仅作备用，真实 redirect URL 由本机配置提供。
- fixture/mock/sandbox/live、设计/实现/测试通过必须区分。

## 变更与验证

- 先检查文件和 Git 状态，保护已有变更，不自行提交、推送或公开仓库。
- 只改当前任务，不预建通用广告平台或插件框架。J runtime 是明确的后续交付项，
  在首版验收/首次推送后实施，不把完整 Pi loop 包装器当作 J 接入。
- 第一阶段交付 fixture + Go/Pi 工具桥 + 只读分析 + React 的运行闭环。
- 不创建/启用真实广告、修改预算/权限，除非当前任务另有明确授权及产品审批。
- 覆盖成功、拒绝、缺失/部分数据、取消/超时；未知写结果禁止盲重试。
- 产品工程已初始化；当前可运行 CLI/Web 与本地 MAPI wire tests，真实 MAPI/J 状态见
  验收记录，不把 HTTP fake 推断成真实账户可用。

当前构建与测试：

```sh
make cli
make test
./bin/ad-agent inspect
```

真实模型探针命令见 `docs/development-readiness.md`，会使用用户 ChatGPT 额度。
探针通过不代表 Go bridge、分析子代理、React 或 MAPI 已通过验收。
