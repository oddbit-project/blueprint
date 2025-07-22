# Next.js API Server with CSRF Protection

This sample demonstrates a production-ready API server built with Blueprint that integrates seamlessly with Next.js 
applications, providing CSRF protection.

## Features

- **CSRF Protection**: Session-based CSRF tokens for all state-changing operations
- **CORS Configuration**: Properly configured for Next.js development and production
- **Session Management**: Secure session handling with customizable settings
- **RESTful API**: Complete CRUD operations with proper error handling
- **Multiple Content Types**: Support for JSON and form data
- **File Upload**: Example file upload endpoint with CSRF protection

## Running the Server

```bash
cd samples/nextjs-api
go run main.go
```

Server starts at: `http://localhost:8080`

## API Endpoints

### Authentication & Security
- `GET /health` - Health check (no auth required)
- `GET /api/csrf-token` - Get CSRF token (session required)

### User Management
- `GET /api/users` - List all users (no CSRF required)
- `GET /api/users/:id` - Get user by ID (no CSRF required)
- `POST /api/users` - Create user (CSRF required)
- `PUT /api/users/:id` - Update user (CSRF required)
- `DELETE /api/users/:id` - Delete user (CSRF required)

### Generic Data Operations
- `POST /api/data` - Create data (CSRF required)
- `PUT /api/data/:id` - Update data (CSRF required)
- `DELETE /api/data/:id` - Delete data (CSRF required)

### Form & File Handling
- `POST /api/submit` - Form submission (CSRF required)
- `POST /api/upload` - File upload (CSRF required)

## Integration Guide

### 1. Frontend Setup (Next.js)

Install required dependencies:
```bash
npm install js-cookie
npm install --save-dev @types/js-cookie
```

### 2. Environment Configuration

Create `.env.local` in your Next.js project:
```env
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
```

### 3. API Client Implementation

```typescript
// lib/api-client.ts
interface APIClientConfig {
  baseURL: string;
  credentials: RequestCredentials;
}

class APIClient {
  private config: APIClientConfig;
  private csrfToken: string | null = null;

  constructor(config: APIClientConfig) {
    this.config = config;
  }

  async getCSRFToken(): Promise<string> {
    if (this.csrfToken) return this.csrfToken;

    const response = await fetch(`${this.config.baseURL}/api/csrf-token`, {
      credentials: this.config.credentials,
    });

    if (!response.ok) {
      throw new Error('Failed to get CSRF token');
    }

    const data = await response.json();
    this.csrfToken = data.csrf_token;
    return this.csrfToken;
  }

  private async makeRequest<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const url = `${this.config.baseURL}${endpoint}`;
    
    // Add CSRF token for state-changing methods
    if (['POST', 'PUT', 'DELETE', 'PATCH'].includes(options.method || '')) {
      const token = await this.getCSRFToken();
      options.headers = {
        ...options.headers,
        'X-CSRF-Token': token,
      };
    }

    const response = await fetch(url, {
      ...options,
      credentials: this.config.credentials,
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    });

    // Update CSRF token if provided in response
    const newToken = response.headers.get('X-CSRF-Token');
    if (newToken) {
      this.csrfToken = newToken;
    }

    if (!response.ok) {
      // Handle CSRF errors
      if (response.status === 403) {
        this.csrfToken = null; // Clear token and retry
        throw new Error('CSRF_ERROR');
      }
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }

    return response.json();
  }

  async get<T>(endpoint: string): Promise<T> {
    return this.makeRequest<T>(endpoint, { method: 'GET' });
  }

  async post<T>(endpoint: string, data?: any): Promise<T> {
    return this.makeRequest<T>(endpoint, {
      method: 'POST',
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  async put<T>(endpoint: string, data?: any): Promise<T> {
    return this.makeRequest<T>(endpoint, {
      method: 'PUT',
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  async delete<T>(endpoint: string): Promise<T> {
    return this.makeRequest<T>(endpoint, { method: 'DELETE' });
  }
}

export const apiClient = new APIClient({
  baseURL: process.env.NEXT_PUBLIC_API_BASE_URL || 'http://localhost:8080',
  credentials: 'include',
});
```

### 4. React Components Example

```typescript
// components/user-manager.tsx
'use client';

import { useState, useEffect } from 'react';
import { apiClient } from '@/lib/api-client';

interface User {
  id: number;
  name: string;
  email: string;
  role: string;
}

export function UserManager() {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

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
      const response = await apiClient.get<{users: User[]}>('/api/users');
      setUsers(response.users);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load users');
    } finally {
      setLoading(false);
    }
  };

  const createUser = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      setError(null);
      await apiClient.post('/api/users', newUser);
      setNewUser({ name: '', email: '', role: 'user' });
      await loadUsers(); // Reload users
    } catch (err) {
      if (err instanceof Error && err.message === 'CSRF_ERROR') {
        // Retry once on CSRF error
        try {
          await apiClient.post('/api/users', newUser);
          setNewUser({ name: '', email: '', role: 'user' });
          await loadUsers();
        } catch (retryErr) {
          setError('Authentication failed. Please refresh the page.');
        }
      } else {
        setError(err instanceof Error ? err.message : 'Failed to create user');
      }
    }
  };

  const deleteUser = async (userId: number) => {
    try {
      setError(null);
      await apiClient.delete(`/api/users/${userId}`);
      await loadUsers(); // Reload users
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete user');
    }
  };

  if (loading) return <div>Loading...</div>;

  return (
    <div className="space-y-6">
      {/* Create User Form */}
      <form onSubmit={createUser} className="space-y-4 p-4 border rounded">
        <h2 className="text-xl font-bold">Create New User</h2>
        
        <div>
          <label htmlFor="name" className="block text-sm font-medium">Name</label>
          <input
            id="name"
            type="text"
            value={newUser.name}
            onChange={(e) => setNewUser({ ...newUser, name: e.target.value })}
            required
            className="mt-1 block w-full border rounded px-3 py-2"
          />
        </div>

        <div>
          <label htmlFor="email" className="block text-sm font-medium">Email</label>
          <input
            id="email"
            type="email"
            value={newUser.email}
            onChange={(e) => setNewUser({ ...newUser, email: e.target.value })}
            required
            className="mt-1 block w-full border rounded px-3 py-2"
          />
        </div>

        <div>
          <label htmlFor="role" className="block text-sm font-medium">Role</label>
          <select
            id="role"
            value={newUser.role}
            onChange={(e) => setNewUser({ ...newUser, role: e.target.value })}
            className="mt-1 block w-full border rounded px-3 py-2"
          >
            <option value="user">User</option>
            <option value="admin">Admin</option>
          </select>
        </div>

        <button
          type="submit"
          className="bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600"
        >
          Create User
        </button>
      </form>

      {/* Error Display */}
      {error && (
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded">
          {error}
        </div>
      )}

      {/* Users List */}
      <div>
        <h2 className="text-xl font-bold mb-4">Users</h2>
        <div className="space-y-2">
          {users.map((user) => (
            <div key={user.id} className="flex items-center justify-between p-3 border rounded">
              <div>
                <div className="font-medium">{user.name}</div>
                <div className="text-sm text-gray-600">{user.email}</div>
                <div className="text-xs text-gray-500">Role: {user.role}</div>
              </div>
              <button
                onClick={() => deleteUser(user.id)}
                className="bg-red-500 text-white px-3 py-1 rounded text-sm hover:bg-red-600"
              >
                Delete
              </button>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
```

## Testing the Integration

### 1. Manual Testing with curl

```bash
# Get CSRF token
curl -c cookies.txt http://localhost:8080/api/csrf-token

# Extract token (Linux/Mac)
TOKEN=$(curl -s -c cookies.txt http://localhost:8080/api/csrf-token | jq -r '.csrf_token')

# Create user with CSRF token
curl -b cookies.txt -H "X-CSRF-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -X POST http://localhost:8080/api/users \
  -d '{"name":"Test User","email":"test@example.com"}'

# Try without token (should fail)
curl -H "Content-Type: application/json" \
  -X POST http://localhost:8080/api/users \
  -d '{"name":"Hacker","email":"hacker@example.com"}'
```

### 2. Browser Testing

1. Start the Go server: `go run main.go`
2. Create a Next.js app with the provided components
3. Test form submissions and API calls
4. Verify CSRF protection in browser dev tools


## Security Considerations

- Always use HTTPS in production
- Set appropriate cookie flags (Secure, HttpOnly, SameSite)
- Configure CORS properly for your domains
- Implement rate limiting
- Add request logging and monitoring
- Validate all inputs on the backend
- Use strong session encryption keys

## Common Issues

1. **CORS Errors**: Ensure your Next.js origin is in the allowed origins list
2. **CSRF Token Refresh**: The token changes after each successful request
3. **Session Persistence**: Make sure cookies are being sent with requests
4. **Development vs Production**: Different security settings needed
