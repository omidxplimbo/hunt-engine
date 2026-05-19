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

export const listNucleiTemplates = async (): Promise<NucleiTemplate[]> => {
  const response = await apiClient.get<ListTemplatesResponse>('/nuclei/templates');
  return response.data.data || [];
};

export const getNucleiTemplate = async (
  name: string
): Promise<NucleiTemplate> => {
  const response = await apiClient.get<TemplateResponse>(
    `/nuclei/templates/${encodeURIComponent(name)}`
  );
  return response.data.data;
};

export const saveNucleiTemplate = async (
  payload: UpsertNucleiTemplatePayload
): Promise<NucleiTemplate> => {
  const response = await apiClient.post<TemplateResponse>(
    '/nuclei/templates',
    payload
  );
  return response.data.data;
};

export const validateNucleiTemplate = async (
  payload: ValidateNucleiTemplatePayload
): Promise<NucleiTemplateValidation> => {
  try {
    const response = await apiClient.post<ValidationResponse>(
      '/nuclei/templates/validate',
      payload
    );
    return response.data.data;
  } catch (error) {
    return extractValidationFailure(error);
  }
};

export const deleteNucleiTemplate = async (name: string): Promise<void> => {
  await apiClient.delete(`/nuclei/templates/${encodeURIComponent(name)}`);
};
