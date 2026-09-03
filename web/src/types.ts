export interface Source {
  backend: string;
  environment: string;
  account_id: string;
}
export interface RuntimeConfig {
  runtime: "pi" | "j" | "custom";
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
export interface Metrics {
  spend: string;
  impressions: number;
  clicks: number;
  conversions: string | null;
  revenue: string | null;
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
  before: Entity;
  after: Entity;
  state: string;
  reason: string;
  currency: string;
  spend_increasing: boolean;
  expires_at: string;
  note?: string;
  approved_by?: string;
}
export interface Card {
  id: string;
  type: string;
  annotation?: string;
  report?: Report;
  calculation?: Calculation;
  comparison?: Comparison;
  entities?: Entity[];
  change?: Change;
  suggestions?: string[];
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
}
export interface TurnResult {
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
