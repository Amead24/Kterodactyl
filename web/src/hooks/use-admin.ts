import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { listUsers, deleteUser, createInvite } from '@/api/admin';
import type { InviteRequest } from '@/types/api';

/** Fetch and cache the list of all users. */
export function useUsers() {
  return useQuery({
    queryKey: ['admin', 'users'],
    queryFn: listUsers,
  });
}

/** Delete a user by username. Invalidates user list on success, shows toast. */
export function useDeleteUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (username: string) => deleteUser(username),
    onSuccess: (_data, username) => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'users'] });
      toast.success(`User "${username}" deleted`);
    },
    onError: (error: Error) => {
      toast.error(`Failed to delete user: ${error.message}`);
    },
  });
}

/** Create an invite. Shows success toast with invite token on success. */
export function useCreateInvite() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: InviteRequest) => createInvite(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'users'] });
      toast.success('Invite created successfully');
    },
    onError: (error: Error) => {
      toast.error(`Failed to create invite: ${error.message}`);
    },
  });
}
