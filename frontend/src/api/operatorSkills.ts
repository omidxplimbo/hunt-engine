import { apiClient } from "./client";

export interface OperatorSkill {
  id: number;
  name: string;
  slug: string;
  version: string;
  category: string;
  bug_class?: string;
  description?: string;
  default_risk_level: string;
  default_safety_level: number;
  default_test_level: number;
  default_autonomy_level: number;
  permission_mode: string;
  runtime_backend?: string;
  is_builtin: boolean;
  is_enabled: boolean;
}

export interface OperatorSkillListResponse {
  status: string;
  count: number;
  skills: OperatorSkill[];
}

export interface OperatorTargetSkillProfile {
  id: number;
  created_at: string;
  updated_at: string;
  user_id: number;
  owner_key: string;
  target_id: number;
  is_enabled: boolean;
  permission_mode: string;
  enabled_skill_slugs: string[];
  disabled_skill_slugs: string[];
  preferred_learning_record_ids: number[];
  allowed_runtime_backends: string[];
  budget_defaults: Record<string, unknown>;
  stop_conditions: Record<string, unknown>;
  metadata: Record<string, unknown>;
}

export interface OperatorTargetSkillProfileResponse {
  status: string;
  profile: OperatorTargetSkillProfile;
}

export interface UpdateOperatorTargetSkillProfilePayload {
  is_enabled?: boolean;
  permission_mode?: string;
  enabled_skill_slugs?: string[];
  disabled_skill_slugs?: string[];
  preferred_learning_record_ids?: number[];
  allowed_runtime_backends?: string[];
  budget_defaults?: Record<string, unknown>;
  stop_conditions?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
}

export async function listOperatorSkills(includeDisabled = false) {
  const response = await apiClient.get<OperatorSkillListResponse>("/operator-skills", {
    params: { include_disabled: includeDisabled },
  });
  return response.data;
}

export async function getTargetOperatorSkillProfile(targetId: number) {
  const response = await apiClient.get<OperatorTargetSkillProfileResponse>(
    `/targets/${targetId}/operator-skill-profile`
  );
  return response.data.profile;
}

export async function updateTargetOperatorSkillProfile(
  targetId: number,
  payload: UpdateOperatorTargetSkillProfilePayload
) {
  const response = await apiClient.put<OperatorTargetSkillProfileResponse>(
    `/targets/${targetId}/operator-skill-profile`,
    payload
  );
  return response.data.profile;
}
