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
        this.csrfToken = null; // Clear token to force refresh on retry
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