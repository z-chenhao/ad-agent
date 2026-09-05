# Web interaction and typography

Ad Desk is the operating workspace; Ad Agent is its assistant. These are presentation
rules for the current alpha, not a new component framework.

## Information ownership

- The workspace toolbar owns account identity, data-source status and the single Sandbox
  clock control. The report bar owns the reporting period.
- Each page owns one heading and one primary analysis action. Object-level decisions
  remain near the relevant object; do not mirror them in Current context.
- Current context identifies the next message's scope. It is not another dashboard:
  no metric summary, recommendation card or diagnosis button. Details expand into the
  conversation's existing scroll area, never an independent vertical scroller.
- Conversation turns own their activity, answer and result cards. Tool history remains
  expandable after completion. Scrolling up to read must not be undone by incoming text.
  Show result cards before the growing activity area and the final interpretation; keep
  follow-up suggestions after the answer. Updates replace cards in place, including
  out-of-order tool completion and saved-event replay. Public commentary streams in
  activity, never as a duplicate answer. Do not narrate card rendering in the answer.
  This is a private layout policy using existing events, not a new message protocol or
  a rewrite of historical prose. We reject automatically deleting model sentences: it
  could remove meaningful qualifications and would hide the actual saved response.
- Runtime/model settings affect the next turn, not conversation identity. Keep history,
  cards, navigation and unsent composer text when saving within the same ad source.
  Refresh reads the configured execution selection, never silently restoring the last
  turn's model. Only an explicit New session or a different bound ad source clears the
  conversation view; retained history is not a new approval or revived dataset handle.
- Activity interleaves public assistant text with tool calls in event order. Preserve
  intermediate commentary on completion/replay; show the final answer only once. Private
  reasoning is never substituted for public commentary. The active tool determines the
  running label; otherwise use neutral Working/Responding, not a stale phase promise.
  Keep setup progress in saved events, not a second Progress updates list.
- Each user message owns a low-emphasis, initially collapsed Context disclosure below
  its bubble, outside execution activity. It reads only that turn's saved `context.bound`
  snapshot (or its currently streaming bound event), never current navigation. Show the
  saved account, object and dates; omit unavailable fields. Old messages without a saved
  snapshot have no fabricated Context entry. Current context remains next-message scope.
- Tool timing measures execution, not model response time. Use milliseconds below one
  second, `<1ms` for a sub-millisecond recorded value, and an advancing timer while running.
  Missing durations stay absent; the turn header shows total elapsed time separately.
- Analysis tool events carry their invoking tool's `parent_id`. Show readable action
  labels under expandable Analysis steps, with the same status and timing conventions
  as main-agent tools. Keep unsuccessful counts visible when collapsed. Do not infer a
  parent for uncorrelated events or append internal role identifiers to action names.
- Metric cards lead with the evidence object's name, then its level and account, with
  the reporting period below. Aggregated queries say All campaigns/ad groups/ads.
  The Agent Application saves display names with the card after bounded, source-checked
  metadata reads. IDs and aggregation scope come from the report/calculation/comparison,
  never navigation or model prose. Unavailable names fall back to IDs; existing records
  are not rewritten. These private presentation labels grant no mutation provenance.
  This keeps scope recognition independent from navigation without creating a new
  entity registry, task framework or compatibility layer.
- Keep exact approval fields, spend warnings, failed outcomes and incomplete-report
  status visible. Explanatory evidence and detailed provenance can use disclosure.
- Briefing is an optional decision summary, not another answer, metric table, task list,
  or approval preview. `present_digest` accepts one topic and at most three distinct
  findings; each requires a judgment (`headline`), supporting evidence (`why`) and a
  concrete next step (`action`). Observation advice specifies a review window or trigger.
  Reject blank fields, literal headline/action repetition, duplicate findings for the
  same subject, and ungrounded references atomically. Semantic evidence quality still
  needs review; schema checks do not verify truth or replace the model's judgment.
  Recommendations are non-interactive body text, not links or composer shortcuts.
  Render the saved server-resolved object as 12 px muted metadata, then the finding as
  14 px semibold, a labeled Next step with 14 px normal body text, and collapsed Evidence
  last. Do not indent every row behind a decorative icon or mute the recommendation.
  Only
  dedicated follow-up suggestions populate the editable composer; they must be complete
  operator requests with a target and task, never copied cautions or implicit approval.
  Do not invent an action button for every finding. Change controls remain in exact
  approval previews, and clicking a suggestion does not send or execute it.
  This is a private alpha presentation contract: keep the existing tool and saved-record
  fields, without a new task/action framework, semantic classifier, or data migration.
  The three-item cap is current presentation policy, not a limit on agent reasoning.
- Do not show the same empty state in both a task queue and a recent-activity panel.
- Product surfaces contain decisions, controls and decision-relevant caveats, not
  engineering rationale. Simulator formulas and validation guidance belong in developer
  documentation, not account/report limitations. Keep the Sandbox source badge, actual
  coverage/backfill/attribution notes, media credits and approval/write restrictions.
  Settings help should explain a choice, prerequisite or consequence, not internal ownership.

## Type and emphasis

| Role                                   | Workspace       | Assistant              |
| -------------------------------------- | --------------- | ---------------------- |
| Page heading                           | 24 px, semibold | No second page heading |
| Section/card heading                   | 16 px, semibold | 14 px, semibold        |
| Body and standard controls             | 14 px           | 14 px                  |
| Metadata, small controls, table labels | 12 px           | 12 px                  |
| Metric values                          | 24 px, semibold | 18 px, semibold        |

Use shared tokens and semantic classes in `web/src/style.css`. Numeric size denotes
measured values, not missing data: unavailable values use body-sized muted text. Use
weight and spacing before adding another size or uppercase label. Muted text must remain
readable; do not substitute low contrast for a clear hierarchy.

Button typography is owned by the shared Button variants. Never override it with an
unlayered `font: inherit` shorthand. Use one visible focus treatment, not stacked outline
and ring treatments. The composer indicates focus on its container.

## Review and regression

Check Today, Campaigns, Creatives, Changes, Settings and Manager scope, plus a populated
conversation. Test long names, expanded context, small viewports, unavailable metrics,
keyboard focus and saved tool activity. Run non-provider browser checks against isolated
local Sandbox state; this does not establish real-model answer quality.

The review uses [visual hierarchy](https://www.nngroup.com/articles/visual-hierarchy-ux-definition/),
[progressive disclosure](https://www.nngroup.com/articles/progressive-disclosure/) and
[WCAG contrast and focus criteria](https://www.w3.org/TR/WCAG22/) as guidance. Automated
checks and screenshot inspection are not a claim of complete WCAG conformance.
