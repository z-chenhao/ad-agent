# 安全说明

Ad Agent 当前是单用户、本地优先的实验性项目，不是多租户广告服务。真实 TikTok
写入默认关闭；fixture、HTTP fake、sandbox 与 live 结果必须明确区分。

## 报告安全问题

请通过仓库的 GitHub Security Advisory 私下报告漏洞，不要在公开 issue 中提交
App Secret、Access Token、OAuth code、operator key、广告账户数据或运行目录内容。
报告中请给出受影响版本、复现条件、预期影响和不含凭证的最小复现。

## 凭证边界

- ChatGPT OAuth 由本机 Pi 用户目录管理，不进入本仓库。
- TikTok App Secret 只通过本机进程环境读取；Access Token 保存在权限为 `0600` 的
  本机 credential 文件中。
- `.data/`、`.env*`、数据库、日志和 runtime checkpoint 均不得提交。
- 授权回调不会回显或记录 `auth_code`、token 或完整 query string。

若凭证意外进入 Git 历史，应先在提供方撤销/轮换，再清理历史；仅删除当前文件不算
完成处置。
