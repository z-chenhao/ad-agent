# 贡献指南

当前接口均为 repo-private、experimental。变更前请阅读 `AGENTS.md`、`AGENT.md` 和
`docs/` 中的技术边界；不要把 runtime、AdBackend 或工具协议扩展成未经验证的通用框架。

提交前运行：

```sh
npm ci --ignore-scripts
make test
go vet ./...
npm run check
npm run build
npm exec prettier -- --check . --ignore-path .gitignore
make test-web
```

普通测试不得依赖 ChatGPT OAuth、TikTok 凭证或外网。真模型、真实 MAPI 和真实写入
必须是显式测试，并在结果中标明环境与授权范围。不得提交凭证、广告账户数据、runtime
checkpoint、构建产物或 `.data/`。
