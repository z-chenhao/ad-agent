import React, { useEffect, useReducer, useState } from "react";
import { createRoot } from "react-dom/client";
import { api, setCSRF, streamTurn } from "./api";
import { emptyLive, reduceEvent } from "./reducer";
import type {
  Account,
  Entity,
  Report,
  Calculation,
  Card,
  Change,
  Session,
  Event,
  Metrics,
} from "./types";
import "./style.css";

const format = (value: string | number | null | undefined, digits = 2) =>
  value == null
    ? "不可用"
    : new Intl.NumberFormat("zh-CN", { maximumFractionDigits: digits }).format(
        Number(value),
      );
const stateName: Record<string, string> = {
  staged: "待审批",
  applying: "执行中",
  applied: "已核对",
  discarded: "已放弃",
  failed: "失败",
  expired: "已过期",
  indeterminate: "结果待核对",
  completed: "已完成",
  cancelled: "已取消",
  budget_exhausted: "达到轮次上限",
  running: "处理中",
  idle: "就绪",
};
const stateText = (state: string) => stateName[state] ?? state;
const toolNames: Record<string, string> = {
  get_advertiser_context: "读取账户",
  list_campaigns: "读取 Campaign",
  list_ad_groups: "读取广告组",
  list_ads: "读取广告",
  get_entity: "核对对象",
  get_performance_report: "读取表现报告",
  run_analysis: "独立分析",
  analysis_calculate: "计算证据",
  analysis_get_dataset: "读取分析快照",
  submit_analysis: "提交分析",
  present_metrics: "展示指标",
  present_entities: "展示对象",
  present_change_preview: "展示草案",
  present_suggestions: "生成后续建议",
  load_skill: "加载操作流程",
  stage_budget_change: "生成预算草案",
  stage_status_change: "生成状态草案",
};

function MetricGrid({
  m,
  roas,
  currency = "USD",
}: {
  m: Metrics;
  roas?: string | null;
  currency?: string;
}) {
  return (
    <div className="metrics">
      <div>
        <span>广告花费 · {currency}</span>
        <strong>{format(m.spend)}</strong>
      </div>
      <div>
        <span>购买价值 · {currency}</span>
        <strong>{format(m.revenue)}</strong>
      </div>
      <div>
        <span>ROAS</span>
        <strong>{format(roas, 3)}</strong>
      </div>
      <div>
        <span>点击 / 曝光</span>
        <strong className="smaller">
          {format(m.clicks, 0)} <small>/ {format(m.impressions, 0)}</small>
        </strong>
      </div>
    </div>
  );
}
function EntityTable({
  entities,
  onSelect,
}: {
  entities: Entity[];
  onSelect?: (e: Entity) => void;
}) {
  return (
    <div className="table-scroll">
      <table>
        <thead>
          <tr>
            <th>对象 / ID</th>
            <th>操作状态</th>
            <th>预算</th>
            <th>预算类型</th>
          </tr>
        </thead>
        <tbody>
          {entities.map((e) => (
            <tr key={e.id}>
              <td>
                {onSelect ? (
                  <button className="text-button" onClick={() => onSelect(e)}>
                    {e.name} <span aria-hidden>→</span>
                  </button>
                ) : (
                  <strong>{e.name}</strong>
                )}
                <code>{e.id}</code>
              </td>
              <td>
                <span
                  className={
                    "status " + (e.status === "ENABLE" ? "enabled" : "")
                  }
                >
                  {e.status === "ENABLE" ? "已启用" : "已停用"}
                </span>
              </td>
              <td>{e.budget == null ? "—" : format(e.budget)}</td>
              <td>
                {e.budget_mode === "BUDGET_MODE_TOTAL"
                  ? "总预算"
                  : e.budget_mode === "BUDGET_MODE_DAY"
                    ? "日预算"
                    : "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {!entities.length && (
        <p className="empty">没有匹配对象，不代表接口失败或花费为零。</p>
      )}
    </div>
  );
}
function CardView({
  card,
  onSuggest,
  onChanges,
}: {
  card: Card;
  onSuggest: (s: string) => void;
  onChanges: () => void;
}) {
  if (card.type === "suggestions")
    return (
      <div className="chips">
        {card.suggestions?.map((s) => (
          <button key={s} onClick={() => onSuggest(s)}>
            {s} <span aria-hidden>↗</span>
          </button>
        ))}
      </div>
    );
  if (card.type === "entities")
    return (
      <section className="card">
        <h3>账户对象</h3>
        <EntityTable entities={card.entities ?? []} />
        {card.annotation && <p className="muted">{card.annotation}</p>}
      </section>
    );
  if (card.type === "change" && card.change) {
    const c = card.change;
    return (
      <section className="card preview">
        <div className="row">
          <h3>变更预览</h3>
          <span className="status">{stateText(c.state)}</span>
        </div>
        <p>{c.before.name}</p>
        <div className="diff">
          <span>
            {c.kind === "budget"
              ? `${c.before.budget} ${c.currency}`
              : c.before.status}
          </span>
          <span aria-label="变更为">→</span>
          <strong>
            {c.kind === "budget"
              ? `${c.after.budget} ${c.currency}`
              : c.after.status}
          </strong>
        </div>
        <p className="muted">
          {c.kind === "budget"
            ? c.before.budget_mode === "BUDGET_MODE_TOTAL"
              ? "总预算"
              : "日预算"
            : "操作状态"}{" "}
          · {c.spend_increasing ? "可能增加花费" : "仍需操作员审批"} ·{" "}
          {c.source.environment}
        </p>
        <button onClick={onChanges}>查看审批队列</button>
      </section>
    );
  }
  if (card.type === "metrics") {
    const c = card.comparison;
    const calc = card.calculation;
    const report = card.report;
    if (c)
      return (
        <section className="card">
          <div className="row">
            <h3>周期对比 · 服务端计算</h3>
            <span className="badge">
              {c.source?.environment ?? "来源见限制"}
            </span>
          </div>
          <p className="muted">
            {c.previous_query?.start_date} — {c.previous_query?.end_date} →{" "}
            {c.current_query?.start_date} — {c.current_query?.end_date} ·{" "}
            {c.timezone}
          </p>
          <div className="diff">
            <span>ROAS {format(c.previous_roas, 3)}</span>
            <span>→</span>
            <strong>{format(c.current_roas, 3)}</strong>
            <small>变化 {format(c.delta_roas, 3)} 点</small>
          </div>
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Campaign ID</th>
                  <th>前期价值</th>
                  <th>当前价值</th>
                  <th>ROAS 贡献点</th>
                </tr>
              </thead>
              <tbody>
                {c.contributions.map((v) => (
                  <tr key={v.entity_id}>
                    <td>
                      <code>{v.entity_id}</code>
                    </td>
                    <td>{format(v.previous.revenue)}</td>
                    <td>{format(v.current.revenue)}</td>
                    <td>{format(v.roas_points, 3)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <details>
            <summary>计算方法与限制</summary>
            <p>{c.method}</p>
            {[...new Set(c.limitations)].map((x) => (
              <p key={x}>{x}</p>
            ))}
          </details>
          {card.annotation && <p className="muted">{card.annotation}</p>}
        </section>
      );
    if (calc || report) {
      const m = calc?.totals ?? report!.totals;
      return (
        <section className="card">
          <h3>表现快照 · {calc?.query?.level ?? report?.query.level}</h3>
          <MetricGrid
            m={m}
            roas={calc?.roas}
            currency={calc?.currency ?? report?.currency}
          />
          <p className="muted">
            {calc?.query?.start_date ?? report?.query.start_date} —{" "}
            {calc?.query?.end_date ?? report?.query.end_date} ·{" "}
            {calc?.timezone ?? report?.timezone}
          </p>
          <details>
            <summary>数据局限</summary>
            {[...new Set(calc?.limitations ?? report?.limitations)].map((x) => (
              <p key={x}>{x}</p>
            ))}
          </details>
        </section>
      );
    }
  }
  return (
    <section className="card warning">
      此组件暂不支持安全展示。记录：<code>{card.id}</code>
    </section>
  );
}
function Login({ onReady }: { onReady: () => void }) {
  const [key, setKey] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  return (
    <main className="login">
      <div className="brand-mark">
        a<span>↗</span>
      </div>
      <p className="eyebrow">LOCAL ADVERTISING WORKSPACE</p>
      <h1>
        让每一次调整
        <br />
        都有证据。
      </h1>
      <p className="muted">
        本地登录后，开始分析和审批。你的模型凭证不会进入浏览器。
      </p>
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setError("");
          try {
            const v = await api<{ csrf: string }>("/login", { key });
            setCSRF(v.csrf);
            setKey("");
            onReady();
          } catch {
            setError(
              "登录失败，请核对本机 operator-key 文件中的密钥。多次失败请稍后再试。",
            );
          } finally {
            setBusy(false);
          }
        }}
      >
        <label htmlFor="operator-key">本机操作员密钥</label>
        <input
          id="operator-key"
          type="password"
          value={key}
          onChange={(e) => setKey(e.target.value)}
          autoComplete="current-password"
          required
        />
        <p className="hint">
          密钥文件位置见启动终端。请勿发送到聊天或提交 Git。
        </p>
        {error && (
          <p role="alert" className="error">
            {error}
          </p>
        )}
        <button className="primary" disabled={busy}>
          {busy ? "验证中…" : "进入工作台 →"}
        </button>
      </form>
      <footer>单用户 · 本地优先 · 所有变更需要明确审批</footer>
    </main>
  );
}

function Workspace() {
  const [page, setPage] = useState("overview");
  const [account, setAccount] = useState<Account>();
  const [error, setError] = useState("");
  const [overview, setOverview] = useState<{
    report: Report;
    calculation: Calculation | null;
  }>();
  const [entities, setEntities] = useState<Entity[]>([]);
  const [level, setLevel] = useState("campaign");
  const [path, setPath] = useState<Entity[]>([]);
  const [changes, setChanges] = useState<Change[]>([]);
  const [confirm, setConfirm] = useState<Change>();
  const [applying, setApplying] = useState(false);
  const [sessionId, setSessionId] = useState(
    () => localStorage.getItem("ad-agent.session") ?? "web",
  );
  const [history, setHistory] = useState<Session>({ id: "web", messages: [] });
  const [message, setMessage] = useState("");
  const [live, dispatch] = useReducer(reduceEvent, emptyLive);
  const [busy, setBusy] = useState(false);
  const [controller, setController] = useState<AbortController>();
  const loadSession = async () => {
    const s = await api<Session>(
      "/session?session_id=" + encodeURIComponent(sessionId),
    );
    setHistory(s);
    const turn = s.messages.at(-1)?.turn_id;
    if (!turn)
      dispatch({
        v: "0",
        type: "client.reset",
        turnId: "",
        seq: 0,
        at: "",
        data: {},
      });
    if (turn) {
      const events = await api<Event[]>(
        `/turns/${encodeURIComponent(turn)}/events?session_id=${encodeURIComponent(sessionId)}`,
      );
      for (const e of events) dispatch(e);
    }
  };
  const loadChanges = async () =>
    setChanges(
      await api<Change[]>(
        "/changes?session_id=" + encodeURIComponent(sessionId),
      ),
    );
  useEffect(() => {
    void api<Account>("/advertisers/current")
      .then(async (a) => {
        setAccount(a);
        const end = a.latest_date;
        const d = new Date(end + "T00:00:00Z");
        d.setUTCDate(d.getUTCDate() - 6);
        setOverview(
          await api(
            `/report?level=campaign&start_date=${d.toISOString().slice(0, 10)}&end_date=${end}`,
          ),
        );
      })
      .catch((e) => setError(String(e)));
  }, []);
  useEffect(() => {
    localStorage.setItem("ad-agent.session", sessionId);
    void loadSession().catch((e) => setError(String(e)));
    void loadChanges().catch((e) => setError(String(e)));
  }, [sessionId]);
  useEffect(() => {
    if (page === "campaigns")
      void api<Entity[]>(
        `/entities/${level}?parent_id=${encodeURIComponent(path.at(-1)?.id ?? "")}`,
      )
        .then(setEntities)
        .catch((e) => setError(String(e)));
    if (page === "changes")
      void loadChanges().catch((e) => setError(String(e)));
  }, [page, level, path, sessionId]);
  const send = async (text: string) => {
    if (busy || !text.trim()) return;
    setError("");
    setMessage("");
    setBusy(true);
    const control = new AbortController();
    setController(control);
    setHistory((h) => ({
      ...h,
      messages: [
        ...h.messages,
        { role: "user", text, turn_id: "pending", status: "running" },
      ],
    }));
    try {
      await streamTurn(sessionId, text, control.signal, dispatch);
    } catch (e) {
      setError(
        control.signal.aborted
          ? "已请求取消。请刷新会话核对保存状态。"
          : String(e),
      );
    } finally {
      setBusy(false);
      setController(undefined);
      await loadSession().catch(() => {});
      await loadChanges().catch(() => {});
    }
  };
  const changeAction = async (c: Change, action: string) => {
    setApplying(true);
    setError("");
    try {
      await api(`/changes/${encodeURIComponent(c.id)}/${action}`, {
        session_id: sessionId,
      });
      setConfirm(undefined);
      await loadChanges();
    } catch {
      setError(
        "操作未确认成功。可能已过期、对象已变化或请求失败，请刷新状态后核对，不要盲目重试。",
      );
    } finally {
      setApplying(false);
    }
  };
  const suggest = (text: string) => {
    setPage("assistant");
    setMessage(text);
  };
  const title: Record<string, string> = {
    overview: "账户概览",
    campaigns: "广告层级",
    assistant: "分析助手",
    changes: "变更审批",
  };
  const pending = changes.filter((c) => c.state === "staged").length;
  return (
    <div className="shell">
      <aside className="sidebar">
        <a
          className="brand"
          href="#"
          onClick={(e) => {
            e.preventDefault();
            setPage("overview");
          }}
        >
          <span className="brand-mark">
            a<span>↗</span>
          </span>
          <span>
            Ad Agent<small>广告工作台</small>
          </span>
        </a>
        <div className="workspace-label">WORKSPACE</div>
        <nav>
          {Object.entries(title).map(([key, label], index) => (
            <button
              key={key}
              aria-current={page === key ? "page" : undefined}
              onClick={() => setPage(key)}
            >
              <span className="nav-icon" aria-hidden>
                {["◫", "▤", "✳", "✓"][index]}
              </span>
              {label}
              {key === "changes" && pending > 0 && (
                <span className="count">{pending}</span>
              )}
            </button>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <span className="connection-dot" /> Pi + Luna
          <small>ChatGPT OAuth · 本地运行</small>
          <p>
            建议可以自动生成，
            <br />
            决策始终由你确认。
          </p>
        </div>
      </aside>
      <main className="main">
        <header className="topbar">
          <div className="crumb">
            工作台 <span>/</span> {title[page]}
          </div>
          <div className="account-mini">
            <span className="badge">FIXTURE · 虚构数据</span>
            <span className="avatar">本</span>
          </div>
        </header>
        <div className="content">
          <div className="page-heading">
            <div>
              <p className="eyebrow">
                {page === "assistant"
                  ? "EVIDENCE BEFORE ACTION"
                  : "YOUR ADS, IN CONTEXT"}
              </p>
              <h1>{title[page]}</h1>
              <p className="muted">
                {account?.name ?? "正在读取账户…"}{" "}
                <span className="separator">·</span> {account?.currency} /{" "}
                {account?.timezone}
              </p>
            </div>
            <button
              className="subtle"
              onClick={() =>
                suggest(
                  "比较最近 7 天与前 7 天表现，指出变化贡献、反证和数据限制。",
                )
              }
            >
              ✳ 开始一次分析
            </button>
          </div>
          <div className="notice">
            <strong>本地演练环境</strong>
            <span>
              官方示例字段 + 明确标记的合成投放数据。不会修改真实广告。
            </span>
            <span className="notice-date">
              数据截至 {account?.latest_date ?? "—"}
            </span>
          </div>
          {error && (
            <div role="alert" className="error alert">
              {error}
              <button className="text-button" onClick={() => setError("")}>
                关闭
              </button>
            </div>
          )}
          {page === "overview" && (
            <>
              <div className="section-heading">
                <h2>最近 7 天</h2>
                <span>
                  {overview?.report.query.start_date} —{" "}
                  {overview?.report.query.end_date} ·{" "}
                  {overview?.report.complete ? "覆盖完整" : "覆盖未确认"}
                </span>
              </div>
              {overview && (
                <MetricGrid
                  m={overview.report.totals}
                  roas={overview.calculation?.roas}
                  currency={overview.report.currency}
                />
              )}
              <div className="overview-grid">
                <section className="card">
                  <div className="row">
                    <h2>把变化，变成可验证的结论</h2>
                    <span className="asterisk" aria-hidden>
                      ✳
                    </span>
                  </div>
                  <p className="muted">
                    从账户到广告，使用同一份事实。独立分析、确定性计算、反证和局限一起呈现。
                  </p>
                  <div className="starter-list">
                    {[
                      "哪个 campaign 拉低了近 7 天 ROAS？与前一期比较。",
                      "按广告组定位购买价值的变化，给出证据和反证。",
                      "查看当前 campaign 状态和预算，不创建草案。",
                    ].map((text, i) => (
                      <button key={text} onClick={() => suggest(text)}>
                        <span>0{i + 1}</span>
                        {text}
                        <b aria-hidden>↗</b>
                      </button>
                    ))}
                  </div>
                </section>
                <section className="card">
                  <p className="eyebrow">HUMAN IN CONTROL</p>
                  <h2>
                    待审批变更 <span className="big-count">{pending}</span>
                  </h2>
                  <p className="muted">
                    模型只创建草案。每次执行都会核对当前值、预算限制和一次性审批。
                  </p>
                  <button onClick={() => setPage("changes")}>
                    查看审批队列 →
                  </button>
                  <hr />
                  <small>账户 ID</small>
                  <code>{account?.id}</code>
                  <p className="hint">操作状态不等于实际交付状态。</p>
                </section>
              </div>
              <section className="card">
                <h3>使用这些数据之前</h3>
                {account?.limitations.map((l) => (
                  <p className="muted" key={l}>
                    {l}
                  </p>
                ))}
              </section>
            </>
          )}
          {page === "campaigns" && (
            <section className="card">
              <div className="row">
                <div className="breadcrumbs">
                  <button
                    className="text-button"
                    onClick={() => {
                      setPath([]);
                      setLevel("campaign");
                    }}
                  >
                    Campaigns
                  </button>
                  {path.map((e, i) => (
                    <React.Fragment key={e.id}>
                      <span>/</span>
                      <button
                        className="text-button"
                        onClick={() => {
                          setPath(path.slice(0, i + 1));
                          setLevel(i === 0 ? "ad_group" : "ad");
                        }}
                      >
                        {e.name}
                      </button>
                    </React.Fragment>
                  ))}
                </div>
                <span className="muted">{entities.length} 个对象</span>
              </div>
              <EntityTable
                entities={entities}
                onSelect={
                  level === "ad"
                    ? undefined
                    : (e) => {
                        setPath([...path, e]);
                        setLevel(level === "campaign" ? "ad_group" : "ad");
                      }
                }
              />
              <p className="hint">
                在分析助手中提出变更请求；浏览页面不直接执行修改。
              </p>
            </section>
          )}
          {page === "assistant" && (
            <div className="assistant-layout">
              <section className="conversation">
                <div className="conversation-bar">
                  <span>
                    会话 <code>{sessionId}</code>
                  </span>
                  <div>
                    <button
                      className="text-button"
                      disabled={busy}
                      onClick={() =>
                        void loadSession().catch((e) => setError(String(e)))
                      }
                    >
                      刷新记录
                    </button>
                    <button
                      className="text-button"
                      disabled={busy}
                      onClick={() => {
                        dispatch({
                          v: "0",
                          type: "client.reset",
                          turnId: "",
                          seq: 0,
                          at: "",
                          data: {},
                        });
                        setSessionId("web-" + crypto.randomUUID().slice(0, 8));
                        setHistory({ id: "", messages: [] });
                      }}
                    >
                      新会话
                    </button>
                  </div>
                </div>
                <div className="messages" aria-live="polite">
                  {history.messages
                    .filter(
                      (m) => m.turn_id !== live.turnId || m.role === "user",
                    )
                    .map((m, i) => (
                      <article
                        key={m.turn_id + "-" + i}
                        className={"message " + m.role}
                      >
                        <small>{m.role === "user" ? "你" : "Ad Agent"}</small>
                        <p>{m.text}</p>
                        {m.status !== "completed" && m.status !== "running" && (
                          <span className="status">{stateText(m.status)}</span>
                        )}
                      </article>
                    ))}
                  {!history.messages.length && !live.text && (
                    <div className="empty welcome">
                      <span className="asterisk">✳</span>
                      <h2>先理解，再调整。</h2>
                      <p>询问表现、浏览对象，或准备一个有边界的变更。</p>
                    </div>
                  )}
                  {live.text && (
                    <article className="message assistant">
                      <small>
                        Ad Agent{" "}
                        <span className="muted">
                          · {stateText(live.status)}
                        </span>
                      </small>
                      <p>{live.text}</p>
                    </article>
                  )}
                  {live.cards.map((c) => (
                    <CardView
                      key={c.id}
                      card={
                        c.change
                          ? {
                              ...c,
                              change:
                                changes.find(
                                  (change) => change.id === c.change?.id,
                                ) ?? c.change,
                            }
                          : c
                      }
                      onSuggest={suggest}
                      onChanges={() => setPage("changes")}
                    />
                  ))}
                </div>
                <form
                  className="composer"
                  onSubmit={(e) => {
                    e.preventDefault();
                    void send(message);
                  }}
                >
                  <label htmlFor="message" className="sr-only">
                    你的广告问题
                  </label>
                  <textarea
                    id="message"
                    placeholder="例如：过去 7 天哪个 campaign 拉低了 ROAS？"
                    value={message}
                    onChange={(e) => setMessage(e.target.value)}
                    maxLength={8000}
                    rows={3}
                  />
                  <div className="row">
                    <small>分析不会自动执行广告变更</small>
                    {busy ? (
                      <button type="button" onClick={() => controller?.abort()}>
                        取消本轮
                      </button>
                    ) : (
                      <button className="primary" disabled={!message.trim()}>
                        发送 ↑
                      </button>
                    )}
                  </div>
                </form>
              </section>
              <aside className="activity card">
                <p className="eyebrow">RUN ACTIVITY</p>
                <h3>执行记录</h3>
                <span className="status">
                  {busy ? "正在处理" : stateText(live.status)}
                </span>
                {live.tools.map((t) => (
                  <div className="activity-item" key={t.id}>
                    <span
                      className={t.ok === false ? "failed-dot" : "activity-dot"}
                    />
                    <div>
                      {toolNames[t.name] ?? t.name}
                      <small>
                        {t.role === "analysis" ? "分析子代理 · " : ""}
                        {t.ok === undefined
                          ? "运行中"
                          : t.ok
                            ? "完成"
                            : "被拒绝 / 失败"}
                      </small>
                    </div>
                  </div>
                ))}
                {!live.tools.length && (
                  <p className="muted">工具调用和公开执行状态会显示在这里。</p>
                )}
                {live.elapsed != null && (
                  <p className="hint">
                    总用时 {(live.elapsed / 1000).toFixed(1)} 秒
                  </p>
                )}
                <hr />
                <p className="hint">
                  这里只显示公开操作，不显示模型私有推理或凭证。
                </p>
              </aside>
            </div>
          )}
          {page === "changes" && (
            <>
              <div className="section-heading">
                <h2>当前会话 · {sessionId}</h2>
                <button
                  onClick={() =>
                    void loadChanges().catch((e) => setError(String(e)))
                  }
                >
                  刷新状态
                </button>
              </div>
              {changes.length === 0 ? (
                <section className="card empty">
                  <span className="asterisk">✓</span>
                  <h2>没有待处理的变更</h2>
                  <p>在分析助手中提出具体调整，先生成草案，再回到这里审批。</p>
                </section>
              ) : (
                changes.map((c) => (
                  <section className="card change-item" key={c.id}>
                    <div className="row">
                      <h3>{c.before.name}</h3>
                      <span
                        className={
                          "status " + (c.state === "applied" ? "enabled" : "")
                        }
                      >
                        {stateText(c.state)}
                      </span>
                    </div>
                    <code>{c.id}</code>
                    <div className="diff">
                      <span>
                        {c.kind === "budget"
                          ? `${c.before.budget} ${c.currency}`
                          : c.before.status}
                      </span>
                      <span>→</span>
                      <strong>
                        {c.kind === "budget"
                          ? `${c.after.budget} ${c.currency}`
                          : c.after.status}
                      </strong>
                    </div>
                    <p>{c.reason}</p>
                    <p className="muted">
                      {c.source.environment} ·{" "}
                      {c.kind === "budget"
                        ? c.before.budget_mode === "BUDGET_MODE_TOTAL"
                          ? "总预算"
                          : "日预算"
                        : "操作状态"}{" "}
                      · 到期 {new Date(c.expires_at).toLocaleString("zh-CN")}
                    </p>
                    {c.spend_increasing && (
                      <p className="risk">
                        此变更可能增加花费，执行前请核对范围。
                      </p>
                    )}
                    {c.note && <p className="hint">{c.note}</p>}
                    <div className="actions">
                      {c.state === "staged" && (
                        <>
                          <button
                            className="primary"
                            disabled={applying}
                            onClick={() => setConfirm(c)}
                          >
                            审批此变更
                          </button>
                          <button
                            disabled={applying}
                            onClick={() => void changeAction(c, "discard")}
                          >
                            放弃草案
                          </button>
                        </>
                      )}
                      {["applying", "indeterminate"].includes(c.state) && (
                        <button
                          disabled={applying}
                          onClick={() => void changeAction(c, "reconcile")}
                        >
                          只读核对结果
                        </button>
                      )}
                    </div>
                  </section>
                ))
              )}
            </>
          )}
          <footer className="workspace-footer">
            Ad Agent <span>事实有来源 · 分析有边界 · 操作有审批</span>
          </footer>
        </div>
      </main>
      {confirm && (
        <div className="modal-backdrop">
          <section
            role="dialog"
            aria-modal="true"
            aria-labelledby="confirm-title"
            className="modal card"
            onKeyDown={(e) => {
              if (e.key === "Escape" && !applying) {
                e.preventDefault();
                setConfirm(undefined);
              }
              if (e.key === "Tab") {
                const buttons = Array.from(
                  e.currentTarget.querySelectorAll<HTMLButtonElement>(
                    "button:not(:disabled)",
                  ),
                );
                const first = buttons[0],
                  last = buttons.at(-1);
                if (e.shiftKey && document.activeElement === first) {
                  e.preventDefault();
                  last?.focus();
                }
                if (!e.shiftKey && document.activeElement === last) {
                  e.preventDefault();
                  first?.focus();
                }
              }
            }}
          >
            <p className="eyebrow">EXPLICIT APPROVAL</p>
            <h2 id="confirm-title">确认这一个变更</h2>
            <p>{confirm.before.name}</p>
            <div className="diff">
              <span>
                {confirm.kind === "budget"
                  ? confirm.before.budget
                  : confirm.before.status}
              </span>
              <span>→</span>
              <strong>
                {confirm.kind === "budget"
                  ? confirm.after.budget
                  : confirm.after.status}
              </strong>
            </div>
            <p className="muted">
              {confirm.source.environment}{" "}
              环境；将重新读取并验证当前状态。按钮不会显示乐观成功，必须等服务端核对。
            </p>
            <div className="actions">
              <button
                className="primary"
                disabled={applying}
                onClick={() => void changeAction(confirm, "apply")}
              >
                {applying ? "执行并核对中…" : "确认并执行此变更"}
              </button>
              <button
                autoFocus
                disabled={applying}
                onClick={() => setConfirm(undefined)}
              >
                返回检查
              </button>
            </div>
          </section>
        </div>
      )}
    </div>
  );
}
function App() {
  const [ready, setReady] = useState(false);
  const [checking, setChecking] = useState(true);
  useEffect(() => {
    void api<{ csrf: string }>("/auth")
      .then((v) => {
        setCSRF(v.csrf);
        setReady(true);
      })
      .catch(() => {})
      .finally(() => setChecking(false));
  }, []);
  if (checking)
    return (
      <main className="login">
        <p>正在连接本地工作台…</p>
      </main>
    );
  return ready ? <Workspace /> : <Login onReady={() => setReady(true)} />;
}
createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
