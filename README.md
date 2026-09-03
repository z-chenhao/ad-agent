# Ad Agent

单用户、本地优先的广告分析与审批助手。Go 拥有广告数据、计算证据、业务工具和审批
账本。现在有两个可运行的 agent runtime：Pi SDK sidecar，或 Go 进程内的 J-agent；
两者都复用本机 ChatGPT OAuth，并显式调用 `gpt-5.6-luna`。React 页面接通同一 Go host。

目前可运行 CLI + React + fixture 闭环，并已实现经过 HTTP fake 验证的只读 TikTok MAPI
adapter 与 callback-only OAuth 接收端。**fixture 是虚构数据，不是真实 TikTok 账户；
没有本机授权凭证时不会发出真实 TikTok 请求，真实写入始终关闭。** 官方 sandbox/live
对账仍需等应用获批和 advertiser 授权。

## 运行

需要 Go 1.26+、Node 24.14+，并事先通过 Pi 完成 ChatGPT OAuth。不会修改 Pi 的全局
默认模型，不自动启动登录。真实模型命令会使用你的 ChatGPT 额度。

```sh
npm ci --ignore-scripts
make cli
./bin/ad-agent inspect
./bin/ad-agent report --level campaign --start 2022-07-11 --end 2022-07-17
./bin/ad-agent chat --message '过去 7 天哪个 campaign 拉低 ROAS？与前 7 天比较，给出反证。'
./bin/ad-agent chat --runtime j --session j-lab --message '读取账户并列出 campaign。'
./bin/ad-agent chat
```

`--json` 输出最终结构化结果，`--events` 输出公开生命周期 NDJSON，`--session` 选择
会话。状态保存在 `.data/`，目录权限必须为 `0700`。不要分享该目录：它包含业务会话
和私有 provider checkpoint。`--runtime` 可选 `pi`（默认）或 `j`；不要在同一个
`--session` 中途切换 runtime，应新建 session。完成 TikTok 授权后，Access Token 也会以 0600 权限保存在
`.data/credentials/`；该初版安全边界是单个 macOS 用户账户，不是多用户 vault。

## 草案与审批

Web 启动：

```sh
make build
./bin/ad-agent serve --addr 127.0.0.1:18080
./bin/ad-agent serve --runtime j --addr 127.0.0.1:18080
```

打开启动输出中的本地地址，用输出提示的 `operator-key` 文件内容登录。密钥只输入
自己的本地页面，不发送到聊天或 Git。主应用禁止监听 3000，避免接入既有回调隧道。
页面支持概览、三级对象浏览、流式分析、审批预览和独立确认；刷新可恢复所选会话。

```sh
./bin/ad-agent chat --message '读取 campaign_example_1，将总预算从 50 USD 改为 55 USD，仅生成草案。'
./bin/ad-agent changes
./bin/ad-agent approve --id CHANGE_ID
./bin/ad-agent discard --id CHANGE_ID
./bin/ad-agent reconcile --id CHANGE_ID
```

把 `CHANGE_ID` 替换为实际生成的 ID。`approve` 是独立操作员动作，聊天中的“批准”
不能执行它。一次只批准一个草案，读回一致才显示 applied；fixture 修改在重启后保留。
未知写结果禁止盲重试，reconcile 只读核对。三个命令是不同操作，不应依次对同一草案执行。

## 数据与边界

- 官方请求示例原始字段单独保存；补充的合成层级和 ad/day 数据明确标记。
- 所有层级由唯一 ad/day 事实表聚合，不分别编造总数；金额用 decimal，缺失不当作零。
- 分析子代理仅访问委派报告，数值由 Go 计算并签发引用；分析不授予修改权限。
- 长期记忆只保存操作员明确要求的稳定偏好、约束和目标，按账户隔离且可删除；不自动
  抽取，不保存凭证、对象 ID、当前指标、权限或广告正文。
- 默认编码工具、扩展和自动上下文发现关闭；模型没有 apply、网络或文件工具。
- MAPI adapter 固定 advertiser，处理 JSON query、分页、业务 code 和缺失指标；live
  backend 不会失败后回退 fixture，也没有 Writer。
- 真实 revenue metric 默认不配置并保持 `null`；需按 advertiser 的 App / Website /
  TikTok Shop 口径显式选择，不能把 App purchase value 当成通用收入。
- TikTok 官方当前说明 advertiser redirect URL 可以登记 localhost。应用获批后从 My Apps
  使用 TikTok 生成的 Advertiser authorization URL；不要自行拼 URL，也不要把验证码、
  `auth_code`、App Secret 或 Access Token 发到聊天。
- 真实 OAuth、MAPI scope、账户授权和指标口径仍须单独验收。

详见 [Agent 合同](AGENT.md)、[技术方案](docs/technical-design-v0.md)、
[Backend 合同](docs/ad-backend-contract.md)、[验收记录](docs/development-readiness.md)、
[fixture 来源](internal/fixture/data/README.md)、[贡献指南](CONTRIBUTING.md)和
[安全说明](SECURITY.md)。

## 测试

```sh
make test
npm exec --workspace=@ad-agent/web -- playwright install chromium
make test-web
# 可选真模型浏览器验证：消耗 ChatGPT 额度，仅修改 fixture
AD_AGENT_LIVE_E2E=1 npm run test:e2e --workspace=@ad-agent/web
# J-agent + Luna 的同一组浏览器验证
AD_AGENT_E2E_RUNTIME=j AD_AGENT_LIVE_E2E=1 npm run test:e2e --workspace=@ad-agent/web
```

普通测试不需要模型或 TikTok 凭证；真实模型、fixture、HTTP fake 与平台验收分开记录。

## TikTok 应用获批后的授权

目前应用仍在审核，无需操作。获批后先启动只提供根路径的回调监听，再把 My Apps 生成的
完整 Advertiser authorization URL 交给 `oauth-start`。命令只替换其中的 `state`，不会
自行推测 TikTok 授权端点：

```sh
export AD_AGENT_TIKTOK_APP_ID='仅保存在本机环境中'
export AD_AGENT_TIKTOK_APP_SECRET='仅保存在本机环境中'
./bin/ad-agent oauth-callback --addr 127.0.0.1:3000 --redirect-url 'http://localhost:3000/'

# 另开一个终端；引号中的 URL 来自 TikTok My Apps
./bin/ad-agent oauth-start --redirect-url 'http://localhost:3000/' \
  --authorization-url 'TIKTOK_MY_APPS_GENERATED_URL'
```

在浏览器打开 `oauth-start` 输出的 URL。TikTok 页面可能要求操作员接收并输入邮件验证码；
验证码只填在 TikTok 页面，不发送给 Ad Agent、聊天或 Git。授权成功后，回调会直接换取
token 并按 advertiser 保存，不会在终端或页面显示 token。若使用已登记的 ngrok URL，
两条命令的 `--redirect-url` 必须逐字一致地改成该 HTTPS 根路径。
