export interface Source {
  backend: string;
  environment: string;
  account_id: string;
}
export interface RuntimeConfig {
  mode: "advertiser" | "manager";
  runtime: "builtin" | "pi" | "codex" | "claude" | "custom";
  writes: boolean;
  sandbox: boolean;
  model: {
    default: ModelSelection;
    options: ModelOption[];
  };
  harness: {
    capabilities: { name: string; description: string; tools: string[] }[];
    grounding: boolean;
    staging_follow_through: boolean;
    stage_shows_preview: boolean;
    close_on_presentation: boolean;
    read_concurrency: boolean;
    partial_presentation: boolean;
    automatic_memory_capture: boolean;
  };
}
export interface SandboxState {
  environment: string;
  account_id: string;
  current_time: string;
  granularity: "hour";
  generated_hours: number;
  fact_count: number;
  seed_start: string;
  seed_end: string;
  limitations: string[];
}
export interface SandboxAdvance {
  previous_time: string;
  advanced_by_hours: number;
  state: SandboxState;
  facts_created: number;
}
export interface ManagerScope {
  id: string;
  name: string;
  accounts: Account[];
}
export interface ManagerAccountSummary {
  account: Account;
  metrics: Metrics;
  roas: string | null;
  complete: boolean;
  limitations: string[];
}
export interface ManagerReport {
  id: string;
  scope_id: string;
  start_date: string;
  end_date: string;
  accounts: ManagerAccountSummary[];
  limitations: string[];
  fetched_at: string;
}
export interface ModelSelection {
  provider: string;
  model: string;
  reasoning: "medium";
  auth_mode: "chatgpt_oauth" | "api_key";
  api?: "anthropic-messages" | "openai-responses" | "openai-completions";
  base_url?: string;
  api_key_env?: string;
  context_window?: number;
  max_output_tokens?: number;
}
export interface ModelOption {
  provider: string;
  model: string;
  label: string;
  auth_mode: "chatgpt_oauth" | "api_key";
  api?: "anthropic-messages" | "openai-responses" | "openai-completions";
  base_url?: string;
  api_key_env?: string;
  context_window?: number;
  max_output_tokens?: number;
}
export interface ViewContext {
  page: "today" | "accounts" | "campaigns" | "creatives" | "changes";
  account_id?: string;
  account_name?: string;
  entity_level?: "campaign" | "ad_group" | "ad";
  entity_id?: string;
  entity_name?: string;
  start_date?: string;
  end_date?: string;
  compare_start?: string;
  compare_end?: string;
}
export interface Account {
  id: string;
  name: string;
  currency: string;
  timezone: string;
  source: Source;
  latest_date: string;
  limitations: string[];
}
export interface Entity {
  id: string;
  account_id: string;
  level: string;
  parent_id?: string;
  name: string;
  status: string;
  budget?: string;
  budget_mode?: string;
  objective?: string;
}
export interface Identity {
  id: string;
  account_id: string;
  name: string;
  kind: string;
  status: string;
}
export interface CreativeAsset {
  id: string;
  account_id: string;
  name: string;
  kind: string;
  status: string;
  review_status: string;
  width?: number;
  height?: number;
  duration_ms?: number;
  updated_at?: string;
}
export interface AdDetail {
  ad: Entity;
  identity?: Identity;
  creative?: CreativeAsset;
  primary_text?: string;
  call_to_action?: string;
  destination_url?: string;
  format?: string;
  media?: {
    kind: "image" | "video";
    preview_url: string;
    poster_url?: string;
    attribution?: string;
    source_url?: string;
  };
  limitations?: string[];
}
export interface Metrics {
  spend: string;
  impressions: number;
  clicks: number;
  conversions: string | null;
  revenue: string | null;
  reach?: number;
  landing_page_views?: number;
  video_views_2s?: number;
  video_views_6s?: number;
  video_views_complete?: number;
}
export interface Query {
  level: string;
  start_date: string;
  end_date: string;
  entity_id?: string;
}
export interface Report {
  id: string;
  source: Source;
  query: Query;
  currency: string;
  timezone: string;
  attribution: string;
  complete: boolean;
  limitations: string[];
  rows: { entity_id: string; date: string; metrics: Metrics }[];
  totals: Metrics;
}
export interface Calculation {
  id: string;
  source: Source;
  query: Query;
  currency: string;
  timezone: string;
  totals: Metrics;
  roas: string | null;
  ranking: {
    entity_id: string;
    metrics: Metrics;
    roas: string | null;
    spend_share: string | null;
  }[];
  limitations: string[];
  method: string;
}
export interface Comparison {
  id: string;
  source: Source;
  current_query: Query;
  previous_query: Query;
  currency: string;
  timezone: string;
  previous: Metrics;
  current: Metrics;
  previous_roas: string | null;
  current_roas: string | null;
  delta_roas: string | null;
  contributions: {
    entity_id: string;
    previous: Metrics;
    current: Metrics;
    roas_points: string | null;
  }[];
  limitations: string[];
  method: string;
}
export interface Change {
  id: string;
  session_id: string;
  source: Source;
  kind: string;
  before?: Entity;
  after?: Entity;
  parent?: Entity;
  create?: {
    level: string;
    parent_id?: string;
    name: string;
    status: string;
    budget?: string;
    budget_mode?: string;
    objective?: string;
  };
  created?: Entity;
  operation?: {
    request: { kind: string } & Record<string, unknown>;
    lines: {
      resource: string;
      id?: string;
      name?: string;
      field: string;
      before?: string;
      after: string;
    }[];
    precondition_hash: string;
    spend_increasing: boolean;
  };
  operation_outcome?: {
    state: string;
    request_ids?: string[];
    resources?: { kind: string; id: string; name?: string }[];
    message?: string;
  };
  state: string;
  reason: string;
  currency: string;
  spend_increasing: boolean;
  created_at: string;
  expires_at: string;
  approved_at?: string;
  note?: string;
  approved_by?: string;
}
export interface Memory {
  id: string;
  key?: string;
  kind: "preference" | "constraint" | "goal";
  text: string;
  created_at: string;
}
export interface Card {
  metric_scope?: {
    account_id: string;
    account_name?: string;
    level: string;
    entity_id?: string;
    entity_name?: string;
  };
  id: string;
  type: string;
  annotation?: string;
  report?: Report;
  calculation?: Calculation;
  comparison?: Comparison;
  entities?: Entity[];
  change?: Change;
  suggestions?: string[];
  digest?: {
    title: string;
    items: {
      kind: "opportunity" | "warning" | "delivery" | "measurement" | "change";
      headline: string;
      why?: string;
      action?: string;
      entity?: Entity;
      change?: Change;
    }[];
  };
  pending?: boolean;
}
export interface Message {
  role: string;
  text: string;
  turn_id: string;
  status: string;
}
export interface Session {
  id: string;
  messages: Message[];
  model: ModelSelection;
}
export interface TurnResult {
  error_code?: string;
  turn_id: string;
  session_id: string;
  status: string;
  text: string;
  cards: Card[];
  elapsed_ms: number;
  usage: {
    input: number;
    output: number;
    cache_read: number;
    cache_write: number;
  };
}
export interface Event {
  v: string;
  type: string;
  turnId: string;
  seq: number;
  at: string;
  data: unknown;
}
