import { apiClient } from "./client";

export type OperatorLearningScope =
  | "target"
  | "project"
  | "user_global"
  | "organization_global";

export type OperatorLearningSource =
  | "user_confirmed"
  | "skill_result"
  | "operator_inference";

export type OperatorLearningStatus =
  | "active"
  | "disabled"
  | "superseded";

export interface OperatorLearningRecord {
  id: number;
  created_at: string;
  updated_at: string;
  user_id: number;
  owner_key?: string;
  scope: OperatorLearningScope;
  source: OperatorLearningSource;
  status: OperatorLearningStatus;
  project_key?: string;
  target_id?: number | null;
  title: string;
  summary?: string;
  content?: string;
  bug_class?: string;
  skill_slug?: string;
  applies_to?: unknown[];
  trigger_signals?: unknown[];
  methodology?: Record<string, unknown>;
  constraints?: Record<string, unknown>;
  execution_hints?: Record<string, unknown>;
  evidence_json?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
  confidence: number;
  use_count: number;
  last_used_at?: string | null;
  promoted_from_observation_id?: number | null;
}

export interface OperatorLearningListParams {
  scope?: OperatorLearningScope | "";
  source?: OperatorLearningSource | "";
  status?: OperatorLearningStatus | "";
  bug_class?: string;
  skill_slug?: string;
  target_id?: number | string;
  project_key?: string;
  limit?: number;
  offset?: number;
}

export interface OperatorLearningListResponse {
  status: string;
  count: number;
  limit: number;
  offset: number;
  learning: OperatorLearningRecord[];
}

export async function listOperatorLearningRecords(params: OperatorLearningListParams = {}) {
  const response = await apiClient.get<OperatorLearningListResponse>("/operator-learning", {
    params,
  });
  return response.data;
}
export interface CreateOperatorLearningRecordPayload {
  scope?: OperatorLearningScope;
  source?: OperatorLearningSource;
  status?: OperatorLearningStatus;
  project_key?: string;
  target_id?: number | null;
  title: string;
  summary?: string;
  content?: string;
  bug_class?: string;
  skill_slug?: string;
  applies_to?: unknown[];
  trigger_signals?: unknown[];
  methodology?: Record<string, unknown>;
  constraints?: Record<string, unknown>;
  execution_hints?: Record<string, unknown>;
  evidence_json?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
  confidence?: number;
}

export interface OperatorLearningRecordResponse {
  status: string;
  learning: OperatorLearningRecord;
}

export async function createOperatorLearningRecord(payload: CreateOperatorLearningRecordPayload) {
  const response = await apiClient.post<OperatorLearningRecordResponse>(
    "/operator-learning",
    payload
  );
  return response.data;
}

export type UpdateOperatorLearningRecordPayload = Partial<CreateOperatorLearningRecordPayload>;

export async function updateOperatorLearningRecord(
  id: number,
  payload: UpdateOperatorLearningRecordPayload
) {
  const response = await apiClient.patch<OperatorLearningRecordResponse>(
    `/operator-learning/${id}`,
    payload
  );
  return response.data;
}

export async function deleteOperatorLearningRecord(id: number) {
  const response = await apiClient.delete<{ status: string; deleted: boolean; id: number }>(
    `/operator-learning/${id}`
  );
  return response.data;
}
