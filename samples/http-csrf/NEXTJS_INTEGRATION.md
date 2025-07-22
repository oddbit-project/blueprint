# Next.js Integration with Blueprint CSRF Protection

This guide shows how to integrate Blueprint's CSRF protection with a Next.js frontend application.

## Architecture Overview

```
Next.js Frontend ←→ Blueprint Go Backend (with CSRF)
```

The Blueprint backend provides:
- Session-based CSRF tokens
- API endpoints protected by CSRF middleware
- Token refresh on successful requests

The Next.js frontend needs to:
- Fetch CSRF tokens from the backend
- Include tokens in API requests
- Handle token refresh/rotation

## Backend Configuration

### 1. API Server Setup

Create a dedicated API server that serves your Next.js app:

```go
// samples/nextjs-api/main.go
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/oddbit-project/blueprint/provider/httpserver/security"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
	"net/http"
	"os"
)

func main() {
	log.Configure(log.NewDefaultConfig())
	logger := log.New("nextjs-api")

	srvConfig := httpserver.NewServerConfig()
	srvConfig.Host = "localhost"
	srvConfig.Port = 8080 // Different port from Next.js (usually 3000)
	srvConfig.Debug = true

	server, err := httpserver.NewServer(srvConfig, logger)
	if err != nil {
		logger.Fatal(err, "could not start http server")
		os.Exit(1)
	}

	// Setup sessions
	sessionConfig := session.NewConfig()
	sessionConfig.Secure = false // Set to true in production with HTTPS
	sessionConfig.SameSite = http.SameSiteLaxMode // Important for cross-origin
	server.UseSession(sessionConfig, nil, logger)

	// CORS middleware for Next.js integration
	corsCfg := security.NewCorsConfig()
	corsCfg.AllowOrigins = []string{
		"http://localhost:3000",
		"http://localhost:3001",
		"https://your-app.vercel.app", // Add your production domain
	}
	server.AddMiddleware(security.CORSMiddleware(corsCfg))

	// Apply CSRF protection to state-changing routes
	server.Route().Use(security.CSRFProtection())

	// CSRF token endpoint (exempt from CSRF protection)
	server.Route().GET("/api/csrf-token", func(c *gin.Context) {
		sess := session.Get(c)
		if sess == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Session error"})
			return
		}

		token := security.GenerateCSRFToken(c)
		sess.Set("_csrf", token)

		c.JSON(http.StatusOK, gin.H{
			"csrf_token": token,
		})
	})

	// Protected API endpoints
	api := server.Route().Group("/api")
	{
		api.POST("/users", createUser)
		api.PUT("/users/:id", updateUser)
		api.DELETE("/users/:id", deleteUser)
		api.POST("/data", handleData)
	}

	server.Start()
}

func createUser(c *gin.Context) {
	var user struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Your user creation logic here
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"user":    user,
		"id":      123, // Generated ID
	})
}

func updateUser(c *gin.Context) {
	userID := c.Param("id")
	
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user_id": userID,
		"updates": updates,
	})
}

func deleteUser(c *gin.Context) {
	userID := c.Param("id")
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"deleted_user_id": userID,
	})
}

func handleData(c *gin.Context) {
	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"received": data,
	})
}
```

## Next.js Frontend Integration

### 1. CSRF Context Provider

Create a context to manage CSRF tokens:

```typescript
// lib/csrf-context.tsx
'use client';

import React, { createContext, useContext, useState, useEffect } from 'react';

interface CSRFContextType {
  token: string | null;
  refreshToken: () => Promise<void>;
  isLoading: boolean;
}

const CSRFContext = createContext<CSRFContextType | undefined>(undefined);

export function CSRFProvider({ children }: { children: React.ReactNode }) {
  const [token, setToken] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const refreshToken = async () => {
    try {
      setIsLoading(true);
      const response = await fetch('http://localhost:8080/api/csrf-token', {
        credentials: 'include', // Important: includes session cookies
      });
      
      if (response.ok) {
        const data = await response.json();
        setToken(data.csrf_token);
      } else {
        console.error('Failed to fetch CSRF token');
        setToken(null);
      }
    } catch (error) {
      console.error('Error fetching CSRF token:', error);
      setToken(null);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    refreshToken();
  }, []);

  return (
    <CSRFContext.Provider value={{ token, refreshToken, isLoading }}>
      {children}
    </CSRFContext.Provider>
  );
}

export function useCSRF() {
  const context = useContext(CSRFContext);
  if (context === undefined) {
    throw new Error('useCSRF must be used within a CSRFProvider');
  }
  return context;
}
```

### 2. API Client with CSRF Support

Create an API client that handles CSRF tokens:

```typescript
// lib/api-client.ts
class APIClient {
  private baseURL: string;
  private csrfToken: string | null = null;

  constructor(baseURL: string = 'http://localhost:8080') {
    this.baseURL = baseURL;
  }

  setCSRFToken(token: string | null) {
    this.csrfToken = token;
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const url = `${this.baseURL}${endpoint}`;
    
    const config: RequestInit = {
      credentials: 'include', // Include session cookies
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    };

    // Add CSRF token for state-changing methods
    if (['POST', 'PUT', 'DELETE', 'PATCH'].includes(options.method || 'GET')) {
      if (this.csrfToken) {
        (config.headers as Record<string, string>)['X-CSRF-Token'] = this.csrfToken;
      }
    }

    const response = await fetch(url, config);

    // Update CSRF token if provided in response
    const newToken = response.headers.get('X-CSRF-Token');
    if (newToken) {
      this.csrfToken = newToken;
    }

    if (!response.ok) {
      throw new Error(`API Error: ${response.status}`);
    }

    return response.json();
  }

  async get<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, { method: 'GET' });
  }

  async post<T>(endpoint: string, data?: any): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'POST',
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  async put<T>(endpoint: string, data?: any): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'PUT',
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  async delete<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, { method: 'DELETE' });
  }
}

export const apiClient = new APIClient();
```

### 3. Custom Hook for API Calls

Create a hook that integrates CSRF tokens with API calls:

```typescript
// hooks/use-api.ts
'use client';

import { useState } from 'react';
import { useCSRF } from '@/lib/csrf-context';
import { apiClient } from '@/lib/api-client';

export function useAPI() {
  const { token, refreshToken } = useCSRF();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Update API client with current CSRF token
  if (token) {
    apiClient.setCSRFToken(token);
  }

  const execute = async <T>(
    apiCall: () => Promise<T>,
    retryOnCSRFError = true
  ): Promise<T | null> => {
    setLoading(true);
    setError(null);

    try {
      const result = await apiCall();
      return result;
    } catch (err) {
      const error = err as Error;
      
      // Retry once if CSRF error (403) and retry is enabled
      if (retryOnCSRFError && error.message.includes('403')) {
        try {
          await refreshToken();
          apiClient.setCSRFToken(token);
          const result = await apiCall();
          return result;
        } catch (retryErr) {
          setError('Authentication failed');
          return null;
        }
      }
      
      setError(error.message);
      return null;
    } finally {
      setLoading(false);
    }
  };

  return { execute, loading, error };
}
```

### 4. Example Components

User management component with CSRF protection:

```typescript
// components/user-form.tsx
'use client';

import { useState } from 'react';
import { useAPI } from '@/hooks/use-api';
import { apiClient } from '@/lib/api-client';

export function UserForm() {
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const { execute, loading, error } = useAPI();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    const result = await execute(() =>
      apiClient.post('/api/users', { name, email })
    );

    if (result) {
      console.log('User created:', result);
      setName('');
      setEmail('');
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div>
        <label htmlFor="name">Name:</label>
        <input
          id="name"
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
          className="border rounded px-2 py-1"
        />
      </div>
      
      <div>
        <label htmlFor="email">Email:</label>
        <input
          id="email"
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
          className="border rounded px-2 py-1"
        />
      </div>

      <button
        type="submit"
        disabled={loading}
        className="bg-blue-500 text-white px-4 py-2 rounded disabled:opacity-50"
      >
        {loading ? 'Creating...' : 'Create User'}
      </button>

      {error && (
        <div className="text-red-500">Error: {error}</div>
      )}
    </form>
  );
}
```

### 5. App Setup

Setup your Next.js app with CSRF protection:

```typescript
// app/layout.tsx
import { CSRFProvider } from '@/lib/csrf-context';

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body>
        <CSRFProvider>
          {children}
        </CSRFProvider>
      </body>
    </html>
  );
}
```

```typescript
// app/page.tsx
import { UserForm } from '@/components/user-form';

export default function Home() {
  return (
    <main className="container mx-auto p-4">
      <h1 className="text-2xl font-bold mb-4">Next.js + Blueprint CSRF Demo</h1>
      <UserForm />
    </main>
  );
}
```

## Key Integration Points

### 1. Session Cookies
- Backend must allow credentials in CORS
- Frontend must use `credentials: 'include'`
- Session cookies carry CSRF tokens

### 2. CSRF Token Flow
1. Frontend fetches token from `/api/csrf-token`
2. Token stored in React context
3. Token sent in `X-CSRF-Token` header
4. Backend validates and refreshes token
5. New token returned in response headers

### 3. Error Handling
- 403 responses indicate CSRF failure
- Automatic token refresh and retry
- User feedback for persistent failures

### 4. Production Considerations
- Use HTTPS in production
- Set secure cookie flags
- Configure proper CORS origins
- Implement rate limiting
- Add request logging

## Testing the Integration

1. Start the Go backend:
```bash
cd samples/nextjs-api
go run main.go
```

2. Start Next.js dev server:
```bash
npx create-next-app@latest my-app
cd my-app
npm run dev
```

3. Test CSRF protection:
- Form submissions should work with tokens
- Direct API calls without tokens should fail
- Token refresh should work automatically

This integration provides robust CSRF protection while maintaining a smooth user experience in your Next.js application.