'use client';

import { useState, useEffect } from 'react';
import { apiClient } from '@/lib/api-client';

interface User {
  id: number;
  name: string;
  email: string;
  role: string;
}

interface APIResponse<T> {
  success: boolean;
  users?: T[];
  total?: number;
  error?: { message: string };
}

export function UserManager() {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [submitLoading, setSubmitLoading] = useState(false);

  // Form state
  const [newUser, setNewUser] = useState({
    name: '',
    email: '',
    role: 'user',
  });

  useEffect(() => {
    loadUsers();
  }, []);

  const loadUsers = async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await apiClient.get<APIResponse<User>>('/api/users');
      if (response.success && response.users) {
        setUsers(response.users);
      } else {
        setError('Failed to load users');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load users');
    } finally {
      setLoading(false);
    }
  };

  const createUser = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      setSubmitLoading(true);
      setError(null);
      
      const response = await apiClient.post('/api/users', newUser);
      console.log('User created:', response);
      
      setNewUser({ name: '', email: '', role: 'user' });
      await loadUsers(); // Reload users
    } catch (err) {
      if (err instanceof Error && err.message === 'CSRF_ERROR') {
        // Retry once on CSRF error
        try {
          const response = await apiClient.post('/api/users', newUser);
          console.log('User created on retry:', response);
          setNewUser({ name: '', email: '', role: 'user' });
          await loadUsers();
        } catch (retryErr) {
          setError('Authentication failed. Please refresh the page.');
        }
      } else {
        setError(err instanceof Error ? err.message : 'Failed to create user');
      }
    } finally {
      setSubmitLoading(false);
    }
  };

  const deleteUser = async (userId: number) => {
    try {
      setError(null);
      await apiClient.delete(`/api/users/${userId}`);
      await loadUsers(); // Reload users
    } catch (err) {
      console.log('Delete error:', err);
      if (err instanceof Error && err.message === 'CSRF_ERROR') {
        // Retry once on CSRF error
        try {
          console.log('Retrying delete for user:', userId);
          await apiClient.delete(`/api/users/${userId}`);
          await loadUsers();
        } catch (retryErr) {
          console.log('Retry error:', retryErr);
          // If retry fails with "Invalid user ID", user might have been deleted by another request
          if (retryErr instanceof Error && retryErr.message.includes('Invalid user ID')) {
            console.log('User might have been deleted already, refreshing list');
            await loadUsers();
          } else {
            setError(`Authentication failed. Retry error: ${retryErr instanceof Error ? retryErr.message : 'Unknown error'}`);
          }
        }
      } else {
        setError(err instanceof Error ? err.message : 'Failed to delete user');
      }
    }
  };

  return (
    <div className="space-y-6">
      {/* Create User Form */}
      <div className="bg-white p-6 rounded-lg shadow">
        <h2 className="text-xl font-bold mb-4">Create New User</h2>
        
        <form onSubmit={createUser} className="space-y-4">
          <div>
            <label htmlFor="name" className="block text-sm font-medium text-gray-700">
              Name
            </label>
            <input
              id="name"
              type="text"
              value={newUser.name}
              onChange={(e) => setNewUser({ ...newUser, name: e.target.value })}
              required
              className="mt-1 block w-full border border-gray-300 rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div>
            <label htmlFor="email" className="block text-sm font-medium text-gray-700">
              Email
            </label>
            <input
              id="email"
              type="email"
              value={newUser.email}
              onChange={(e) => setNewUser({ ...newUser, email: e.target.value })}
              required
              className="mt-1 block w-full border border-gray-300 rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div>
            <label htmlFor="role" className="block text-sm font-medium text-gray-700">
              Role
            </label>
            <select
              id="role"
              value={newUser.role}
              onChange={(e) => setNewUser({ ...newUser, role: e.target.value })}
              className="mt-1 block w-full border border-gray-300 rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value="user">User</option>
              <option value="admin">Admin</option>
            </select>
          </div>

          <button
            type="submit"
            disabled={submitLoading}
            className="bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {submitLoading ? 'Creating...' : 'Create User'}
          </button>
        </form>
      </div>

      {/* Error Display */}
      {error && (
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded">
          <strong>Error:</strong> {error}
        </div>
      )}

      {/* Users List */}
      <div className="bg-white p-6 rounded-lg shadow">
        <div className="flex justify-between items-center mb-4">
          <h2 className="text-xl font-bold">Users</h2>
          <button
            onClick={loadUsers}
            disabled={loading}
            className="bg-gray-500 text-white px-3 py-1 rounded text-sm hover:bg-gray-600 disabled:opacity-50"
          >
            {loading ? 'Loading...' : 'Refresh'}
          </button>
        </div>

        {loading ? (
          <div className="text-center py-4">Loading users...</div>
        ) : (
          <div className="space-y-3">
            {users.length === 0 ? (
              <div className="text-center py-4 text-gray-500">No users found</div>
            ) : (
              users.map((user) => (
                <div key={user.id} className="flex items-center justify-between p-4 border border-gray-200 rounded-lg">
                  <div>
                    <div className="font-medium text-gray-900">{user.name}</div>
                    <div className="text-sm text-gray-600">{user.email}</div>
                    <div className="text-xs text-gray-500 uppercase">Role: {user.role}</div>
                  </div>
                  <button
                    onClick={() => deleteUser(user.id)}
                    className="bg-red-500 text-white px-3 py-1 rounded text-sm hover:bg-red-600"
                  >
                    Delete
                  </button>
                </div>
              ))
            )}
          </div>
        )}
      </div>
    </div>
  );
}