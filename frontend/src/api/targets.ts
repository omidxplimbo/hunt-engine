import { apiClient } from './client';
import type { TargetResponse } from '../types/target';

export const getTargets = async (page = 1, limit = 50) => {
  // محاسبه آفست بر اساس شماره صفحه
  const offset = (page - 1) * limit;
  
  const response = await apiClient.get<TargetResponse>('/targets', {
    params: { limit, offset },
  });
  
  return response.data;
};