import { apiClient } from './client';

export type NucleiTemplatePlacement =
  | 'root'
  | 'shared'
  | 'safe'
  | 'fast'
  | 'exposure'
  | 'balanced'
  | 'misconfig'
  | 'cves'
  | 'cves-light'
  | 'full'
  | 'custom';

export const NUCLEI_TEMPLATE_PLACEMENTS: NucleiTemplatePlacement[] = [
  'root',
  'shared',
  'safe',
  'fast',
  'exposure',
  'balanced',
  'misconfig',
  'cves',
  'cves-light',
  'full',
  'custom',
];

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
}

export interface NucleiTemplateDraftRequest {
  name: string;
  title: string;
  description: string;
  severity: 'info' | 'low' | 'medium' | 'high' | 'critical';
  tags: string[];
  method: string;
  path: string;
  matcher_type: 'word' | 'regex' | 'status';
  matcher_part: string;
  matcher_value: string;
  validate?: boolean;
}

export interface NucleiTemplateDraft {
  name: string;
  content: string;
  validation?: NucleiTemplateValidation;
  draft_only: boolean;
  requires_human_review: boolean;
  saved: boolean;
}

export interface NucleiTemplateStrategyActions {
  can_select_profile?: boolean;
  can_select_builtin_tags?: boolean;
  can_select_custom_groups?: boolean;
  can_auto_save_template?: boolean;
  can_auto_execute_template?: boolean;
  requires_human_approval?: boolean;
}

export interface NucleiTemplateStrategy {
  target_id?: number;
  agent_ready: boolean;
  draft_only: boolean;
  ai_template_drafts_enabled: boolean;
  save_automatically: boolean;
  execute_automatically: boolean;
  recommended_profile?: string;
  recommended_tags?: string[];
  recommended_placements?: string[];
  recommended_template_sets?: string[];
  signals?: string[];
  rationale?: string[];
  suggested_draft_request?: Partial<NucleiTemplateDraftRequest>;
  allowed_actions?: NucleiTemplateStrategyActions;
  generated_draft?: NucleiTemplateDraft;
}

export interface NucleiTemplateStrategyOptions {
  includeDraft?: boolean;
  validate?: boolean;
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
      'Template validation failed',
  };
};

const responseData = <T,>(response: { data: { data: T } }): T => response.data.data;

export const listNucleiTemplates = async (): Promise<NucleiTemplate[]> => {
  const response = await apiClient.get('/nuclei/templates');
  return response.data.data || [];
};

export const getNucleiTemplate = async (
  name: string,
  placement?: NucleiTemplatePlacement
): Promise<NucleiTemplate> => {
  const response = await apiClient.get(
    `/nuclei/templates/${encodeURIComponent(name)}`,
    { params: placement ? { placement } : undefined }
  );
  return responseData<NucleiTemplate>(response);
};

export const saveNucleiTemplate = async (
  payload: UpsertNucleiTemplatePayload
): Promise<NucleiTemplate> => {
  const response = await apiClient.post('/nuclei/templates', payload);
  return responseData<NucleiTemplate>(response);
};

export const validateNucleiTemplate = async (
  payload: ValidateNucleiTemplatePayload
): Promise<NucleiTemplateValidation> => {
  try {
    const response = await apiClient.post('/nuclei/templates/validate', payload);
    return responseData<NucleiTemplateValidation>(response);
  } catch (error) {
    return extractValidationFailure(error);
  }
};

export const deleteNucleiTemplate = async (
  name: string,
  placement?: NucleiTemplatePlacement
): Promise<void> => {
  await apiClient.delete(`/nuclei/templates/${encodeURIComponent(name)}`, {
    params: placement ? { placement } : undefined,
  });
};

export const getNucleiTemplateDraftStatus = async (): Promise<NucleiTemplateDraftStatus> => {
  const response = await apiClient.get('/nuclei/template-drafts/status');
  return responseData<NucleiTemplateDraftStatus>(response);
};

export const generateNucleiTemplateDraft = async (
  payload: NucleiTemplateDraftRequest
): Promise<NucleiTemplateDraft> => {
  const response = await apiClient.post('/nuclei/template-drafts', payload);
  return responseData<NucleiTemplateDraft>(response);
};

export const getNucleiTemplateStrategy = async (
  targetId: string | number,
  options: NucleiTemplateStrategyOptions = {}
): Promise<NucleiTemplateStrategy> => {
  const response = await apiClient.get(
    `/nuclei/template-drafts/targets/${encodeURIComponent(String(targetId))}/strategy`,
    {
      params: {
        include_draft: options.includeDraft || undefined,
        validate: options.validate || undefined,
      },
    }
  );
  return responseData<NucleiTemplateStrategy>(response);
};
