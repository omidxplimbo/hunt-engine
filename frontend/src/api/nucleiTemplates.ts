import { apiClient } from "./client";

export type NucleiTemplatePlacement =
  | "root"
  | "shared"
  | "safe"
  | "fast"
  | "exposure"
  | "balanced"
  | "misconfig"
  | "cves"
  | "cves-light"
  | "full"
  | "custom";

export interface NucleiTemplate {
  name: string;
  path: string;
  placement?: NucleiTemplatePlacement;
  size_bytes: number;
  updated_at: string;
  content?: string;
}

export interface NucleiTemplateValidation {
  valid: boolean;
  name?: string;
  output?: string;
  error?: string;
}

interface ListTemplatesResponse {
  status: string;
  data: NucleiTemplate[];
  count: number;
}

interface TemplateResponse {
  status: string;
  message?: string;
  data: NucleiTemplate;
}

interface ValidationResponse {
  status: string;
  data: NucleiTemplateValidation;
}

interface ValidationFailureResponse {
  validation?: NucleiTemplateValidation;
  data?: NucleiTemplateValidation;
  message?: string;
  error?: string;
}

export interface UpsertNucleiTemplatePayload {
  name: string;
  content: string;
  placement?: NucleiTemplatePlacement;
  validate?: boolean;
}

export interface ValidateNucleiTemplatePayload {
  name?: string;
  content?: string;
  placement?: NucleiTemplatePlacement;
}

export interface NucleiTemplateDraftStatus {
  enabled: boolean;
  provider: string;
  draft_only: boolean;
  requires_human_review: boolean;
  save_automatically: boolean;
  feature_enabled?: boolean;
  environment_enabled?: boolean;
  disabled_reason?: string;
  scope?: string;
  owner_key?: string;
}

export interface GenerateNucleiTemplateDraftPayload {
  name: string;
  title: string;
  description: string;
  severity: string;
  tags: string[];
  method: string;
  path: string;
  matcher_type: string;
  matcher_part: string;
  matcher_value: string;
  validate?: boolean;
}

export interface NucleiTemplateDraftResponse {
  name: string;
  content: string;
  draft_only: boolean;
  requires_human_review: boolean;
  saved: boolean;
  validation?: NucleiTemplateValidation;
}

export interface NucleiTemplateStrategySignal {
  kind: string;
  value: string;
  confidence: string;
  reason: string;
}

export interface NucleiTemplateStrategy {
  agent_ready: boolean;
  draft_only: boolean;
  ai_template_drafts_enabled: boolean;
  save_automatically: boolean;
  execute_automatically: boolean;
  feature_enabled?: boolean;
  environment_enabled?: boolean;
  disabled_reason?: string;
  scope?: string;
  owner_key?: string;
  recommended_profile: string;
  recommended_tags: string[];
  recommended_placements: string[];
  recommended_template_sets: string[];
  signals: NucleiTemplateStrategySignal[];
  rationale: string[];
  suggested_draft_request?: GenerateNucleiTemplateDraftPayload;
  generated_draft?: NucleiTemplateDraftResponse;
  draft_error?: string;
  target?: {
    id: number;
    name: string;
    root_domain: string;
    nuclei_profile: string;
    use_nuclei: boolean;
    live_asset_count: number;
    sample_urls: string[];
    technologies: string[];
    web_servers: string[];
    open_ports: number[];
  };
  allowed_actions?: {
    can_select_profile: boolean;
    can_select_builtin_tags: boolean;
    can_select_custom_groups: boolean;
    can_generate_draft: boolean;
    can_save_template: boolean;
    can_auto_save_template: boolean;
    can_auto_execute_template: boolean;
    requires_human_approval: boolean;
  };
}

const extractValidationFailure = (error: unknown): NucleiTemplateValidation => {
  const maybeError = error as {
    response?: { data?: ValidationFailureResponse };
    message?: string;
  };

  const payload = maybeError.response?.data;
  if (payload?.validation) return payload.validation;
  if (payload?.data) return payload.data;

  return {
    valid: false,
    error:
      payload?.message ||
      payload?.error ||
      maybeError.message ||
      "Template validation failed",
  };
};

export const listNucleiTemplates = async (): Promise<NucleiTemplate[]> => {
  const response =
    await apiClient.get<ListTemplatesResponse>("/nuclei/templates");
  return response.data.data || [];
};

export const getNucleiTemplate = async (
  name: string,
): Promise<NucleiTemplate> => {
  const response = await apiClient.get<TemplateResponse>(
    `/nuclei/templates/${encodeURIComponent(name)}`,
  );
  return response.data.data;
};

export const saveNucleiTemplate = async (
  payload: UpsertNucleiTemplatePayload,
): Promise<NucleiTemplate> => {
  const response = await apiClient.post<TemplateResponse>(
    "/nuclei/templates",
    payload,
  );
  return response.data.data;
};

export const validateNucleiTemplate = async (
  payload: ValidateNucleiTemplatePayload,
): Promise<NucleiTemplateValidation> => {
  try {
    const response = await apiClient.post<ValidationResponse>(
      "/nuclei/templates/validate",
      payload,
    );
    return response.data.data;
  } catch (error) {
    return extractValidationFailure(error);
  }
};

export const deleteNucleiTemplate = async (name: string): Promise<void> => {
  await apiClient.delete(`/nuclei/templates/${encodeURIComponent(name)}`);
};

export const getNucleiTemplateDraftStatus =
  async (): Promise<NucleiTemplateDraftStatus> => {
    const response = await apiClient.get<{
      status: string;
      data: NucleiTemplateDraftStatus;
    }>("/nuclei/template-drafts/status");
    return response.data.data;
  };

export const generateNucleiTemplateDraft = async (
  payload: GenerateNucleiTemplateDraftPayload,
): Promise<NucleiTemplateDraftResponse> => {
  const response = await apiClient.post<{
    status: string;
    data: NucleiTemplateDraftResponse;
  }>("/nuclei/template-drafts", payload);
  return response.data.data;
};

export const getNucleiTemplateStrategy = async (
  targetId: string | number,
  options?: { includeDraft?: boolean; validate?: boolean },
): Promise<NucleiTemplateStrategy> => {
  const params = new URLSearchParams();
  if (options?.includeDraft) params.set("include_draft", "true");
  if (options?.validate) params.set("validate", "true");

  const query = params.toString();
  const response = await apiClient.get<{
    status: string;
    data: NucleiTemplateStrategy;
  }>(
    `/nuclei/template-drafts/targets/${encodeURIComponent(String(targetId))}/strategy${
      query ? `?${query}` : ""
    }`,
  );
  return response.data.data;
};
