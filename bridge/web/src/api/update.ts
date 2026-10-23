import api from './client';

export interface UpdateInfo {
  current: string;
  latest: string;
  has_update: boolean;
  dev: boolean;
  download_url: string;
  error: string;
}

export interface UpdateApplyResult {
  status: 'restarting' | 'failed' | 'up_to_date';
  version?: string;
  error?: string;
  download_url?: string;
}

export const getUpdate = () => api.get<UpdateInfo>('/update');
export const applyUpdate = () => api.post<UpdateApplyResult>('/update/apply');
