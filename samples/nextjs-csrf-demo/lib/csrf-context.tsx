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
      const response = await fetch(`${process.env.NEXT_PUBLIC_API_BASE_URL}/api/csrf-token`, {
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