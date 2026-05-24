import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { restFetch } from '@/lib/api';

export interface AdminUser {
  userId: string;
  email: string;
  name: string;
  role: 'admin' | 'manager' | 'user';
  status: string;
  lastLogin?: string;
}

export interface UsersList {
  items: AdminUser[];
  total: number;
  limit: number;
  offset: number;
}

const QK = ['admin-users'];

export function useUsers(params: { limit?: number; offset?: number } = {}) {
  const q = new URLSearchParams();
  if (params.limit  != null) q.set('limit',  String(params.limit));
  if (params.offset != null) q.set('offset', String(params.offset));
  const s = q.toString();
  return useQuery({
    queryKey: [...QK, params],
    queryFn: () => restFetch<UsersList>(`/api/v1/admin/users${s ? `?${s}` : ''}`),
    staleTime: 30_000,
  });
}

export function useUpdateUserRole() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: AdminUser['role'] }) =>
      restFetch(`/api/v1/admin/users/${encodeURIComponent(userId)}/role`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ role }),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QK }),
  });
}
