'use client';

import { useCSRF } from '@/lib/csrf-context';

export function CSRFStatus() {
  const { token, refreshToken, isLoading } = useCSRF();

  return (
    <div className="bg-blue-50 p-4 rounded-lg border border-blue-200">
      <h3 className="font-semibold text-blue-900 mb-2">CSRF Protection Status</h3>
      
      <div className="space-y-2 text-sm">
        <div className="flex items-center gap-2">
          <span className="font-medium">Status:</span>
          {isLoading ? (
            <span className="text-yellow-600">Loading...</span>
          ) : token ? (
            <span className="text-green-600">✓ Protected</span>
          ) : (
            <span className="text-red-600">✗ Not Protected</span>
          )}
        </div>
        
        {token && (
          <div className="flex items-center gap-2">
            <span className="font-medium">Token:</span>
            <span className="font-mono text-xs bg-gray-100 px-2 py-1 rounded">
              {token.substring(0, 8)}...{token.substring(token.length - 8)}
            </span>
          </div>
        )}
        
        <button
          onClick={refreshToken}
          disabled={isLoading}
          className="bg-blue-500 text-white px-3 py-1 rounded text-xs hover:bg-blue-600 disabled:opacity-50"
        >
          {isLoading ? 'Refreshing...' : 'Refresh Token'}
        </button>
      </div>
    </div>
  );
}