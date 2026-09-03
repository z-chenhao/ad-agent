# 实施进度与验收记录

更新：2026-09-04。无需重新确认架构或重新登录 Pi。项目仍在实施中，不能将
CLI 验证扩大为整个产品完成，也不能把 fixture 结果称为真实 TikTok 接入成功。

## 已固定的决定

- 单用户本地工具；Go host + React；AdBackend 与 runtime 分别替换。
- 首个 runtime：Pi 0.84.4 薄 sidecar，ChatGPT OAuth，显式 gpt-5.6-luna。
- CLI 验证后开发 Web；整体 review 和首次推送后继续实现真实 J-agent runtime。
- 只读分析、草案、操作员审批分离；模型没有 apply 工具。
- 无真实账户时使用明确标记的 fixture，不等待 TikTok 审批，不制造真实投放数据。

## 当前证据

| 项目             | 结果                                                                                     | 证明范围                                                                                                                                      |
| ---------------- | ---------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| Go / Node / Pi   | Go 1.26.2、Node 24.14.1、Pi SDK 0.84.4                                                   | 本次工具链，依赖已锁定                                                                                                                        |
| 编译             | make build 通过                                                                          | Go CLI/HTTP、Pi sidecar 与 React production bundle 可构建                                                                                     |
| Go 测试          | go test -race ./... 通过                                                                 | 领域、fixture、host、store、bridge、HTTP 安全与重启验证                                                                                       |
| TS 测试          | Pi bridge 协议解析与 fencing 测试通过                                                    | 不等于完整 provider 行为覆盖                                                                                                                  |
| 真实 CLI 读取    | 2026-09-04 最新复测 22.9 秒完成账户/campaign 读取、实体卡片与来源披露                    | Go → Pi → Luna → Go 工具 → 续答；未创建草案                                                                                                   |
| 真实 CLI 分析    | 两期报告、独立 child、Go 比较证据、可信卡片，约 93 秒                                    | ROAS 3 → 1.6667；首个 campaign 贡献 -1.3333，控制组 0                                                                                         |
| 真实 CLI 草案    | 同会话恢复、重读对象、50 → 55 USD 总预算草案及预览，约 22 秒                             | 模型只 stage，未修改数据源                                                                                                                    |
| 独立 CLI 审批    | 草案 approved/applied，重启读取预算 55，再次审批被拒绝                                   | fixture-only 写入、持久化与单次审批                                                                                                           |
| React 浏览器测试 | 登录、概览、三级层级、CSRF、事件去重、桌面/手机布局通过                                  | production bundle + loopback host                                                                                                             |
| Web 真模型用例   | 2026-09-04 最新复测 37.1 秒完成 stream → stage → 独立审批 → read-back → reload；5/5 通过 | 完整本地页面闭环；使用 fixture，不代表真实 TikTok 写入                                                                                        |
| MAPI adapter     | HTTP fake 通过                                                                           | account/list/get/report、官方 `AUCTION_*` 层级、30 天 daily 上限、JSON query、分页、429/5xx、业务 code、跨账户和缺失指标；不等于真实权限/口径 |
| OAuth callback   | 本地安全测试和 `oauth-start` CLI smoke 通过                                              | 官方 URL 保留/校验、一次性 hash-only state、token exchange、0600 credential、callback-only mux；尚未跑真实授权                                |
| J-agent / GitHub | 尚未实施 / 尚未创建推送                                                                  | 按交付顺序继续                                                                                                                                |

模型验证仅发送 fixture 数据，使用既有 ChatGPT OAuth。最初探针的 fetch 失败源于
遗漏 HTTP 初始化；正式 sidecar 显式使用 undici 的代理 dispatcher 与 fetch。
没有修改用户全局默认模型、OAuth 或系统代理。

2026-09-04 曾出现一次 provider transport 故障：WebSocket 失败后 SSE 也 `fetch failed`，
应用按 failed 终结且没有工具调用或草案；没有修改代理或 OAuth。随后在相同配置下，锁定
Pi/Luna 的 readiness probe、真实 CLI 读取和 Web 真模型 E2E 均复测通过。该故障保留为
历史证据，说明外部 provider 可暂时不可用；当前验证状态以最新成功为准，不把它推断成
网络永远稳定。

复现：

```sh
make cli
make test
./bin/ad-agent chat --session diagnosis --json --message '以 fixture 最新日期为锚点，比较近 7 天与前 7 天 campaign ROAS，调用分析子代理并展示计算证据，不创建草案。'
```

历史探针可使用仓库锁定 SDK：

```sh
node scripts/check-pi-readiness.mjs "$PWD/node_modules/@earendil-works/pi-coding-agent/dist/index.js"
```

该探针使用固定版本内部初始化入口，仅供诊断；产品 sidecar 不依赖它。

## 回调与隧道

用户已登记一个 HTTPS ngrok 根路径回调；真实地址由本机配置提供，不写入仓库。既有
隧道指向 localhost:3000，该端口不能暴露完整管理页面、文件服务器或调试器。
TikTok 当前官方 [Authorization 文档](https://business-api.tiktok.com/portal/docs?id=1738373141733378)
明确允许最多 10 个 advertiser redirect URL，包括 localhost。因此本地单用户首选
`http://localhost:3000/`，ngrok 仅作为备用；应用
正在审核时不为切换地址重复提交。应用获批与回调可达是两个独立条件。

| 本地监听       | 用途                          | 状态                                                 |
| -------------- | ----------------------------- | ---------------------------------------------------- |
| loopback:8080  | Go 主应用/API，构建后的 React | 已实现；本机当前被其他服务占用时用其他 loopback 端口 |
| loopback:5173  | React 开发服务器              | 已配置；生产验收使用 Go 同源静态资源                 |
| localhost:3000 | callback-only mux             | 已实现并测试；仅授权时显式启用                       |

`oauth-start` 只接受从 TikTok My Apps 复制的 `business-api.tiktok.com` 或 `ads.tiktok.com`
HTTPS 授权 URL，保留 portal 参数并替换一次性 state；不猜测授权 endpoint。回调使用
256-bit 高熵、最长 15 分钟、一次性 state；SQLite 只保存 hash，并绑定连接
意图和准确 redirect URL。拒绝过期、重放和无效 state，不记录/回显 code/token/query；
响应 no-store/no-referrer/CSP。若使用 ngrok，真实授权前关闭请求检查并核对 capture/
retention；localhost 不经过公网隧道，风险面更小。官方 [Authentication 文档](https://business-api.tiktok.com/portal/docs?id=1738373164380162)
规定 `auth_code` 一小时且仅能使用一次；由程序直接换 token。TikTok 页面可能要求广告主
输入邮件验证码，验证码只在 TikTok 页面输入，用户无需向本项目或聊天复制任何确认码。

## 尚需完成的验收

1. CLI：读取、分析、草案、独立审批和恢复已通过；长对话压缩与 provider 故障矩阵留到可靠性阶段压力验收。
2. Harness：显式 memory 生命周期、分析资源预算和 parent/child usage 汇总已完成；仍需以
   golden conversations 调整 grounding/follow-through 策略，不能只靠关键词硬编码意图。
3. MAPI：本地 adapter/callback 已完成；待 app approval 后验证真实 scopes、官方
   Advertiser authorization URL、advertiser binding、报表字段和 Ads Manager 对账。
4. Web：已完成首版同源登录、CSRF、SSE 重放、React 页面、普通及真模型 Playwright E2E；
   MAPI 环境 E2E 受 app approval 阻塞，无障碍审计留到真实产品 UI 阶段。
5. Review：安全/质量审查、依赖许可、CI 和文档命令正在做最终验证；之后创建私有仓库并推送。
6. J runtime：J-agent 真正拥有 model/tool loop，复用业务门禁及 OAuth/Luna。

## 以后需要用户参与

- 真实只读：应用获批后，在 My Apps 取得 TikTok 生成的 Advertiser authorization URL，
  选择 redirect URL 和 advertiser 并由用户本人完成授权。App ID/Secret 仅在本机配置；
  TikTok 邮件验证码仅在 TikTok 页面输入；验证码、`auth_code`、token 不发送到聊天。
  目前无需用户提供确认码。
- 真实写入：明确受控测试对象、预算上限和启停范围，每条单独审批。开发不等于花费授权。
- 远端发布：默认私有仓库，不把创建仓库理解为公开数据或凭证授权。
