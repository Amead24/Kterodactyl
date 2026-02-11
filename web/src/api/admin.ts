import { apiFetch } from '@/api/client';
import type {
  UserResponse,
  InviteRequest,
  InviteResponse,
  ListResponse,
} from '@/types/api';

/** GET /admin/users -- List all registered users. */
export function listUsers(): Promise<ListResponse<UserResponse>> {
  return apiFetch<ListResponse<UserResponse>>('/admin/users');
}

/** DELETE /admin/users/{username} -- Delete a user by username. */
export function deleteUser(username: string): Promise<void> {
  return apiFetch<void>(`/admin/users/${username}`, {
    method: 'DELETE',
  });
}

/** POST /admin/invites -- Create an invite for a new user. */
export function createInvite(data: InviteRequest): Promise<InviteResponse> {
  return apiFetch<InviteResponse>('/admin/invites', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}
