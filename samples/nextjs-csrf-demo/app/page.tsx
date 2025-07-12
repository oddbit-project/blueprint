import { UserManager } from "@/components/user-manager";
import { CSRFStatus } from "@/components/csrf-status";

export default function Home() {
  return (
    <div className="min-h-screen p-8">
      <div className="max-w-4xl mx-auto">
        {/* Header */}
        <header className="text-center mb-8">
          <h1 className="text-3xl font-bold text-gray-900 mb-2">
            Next.js + Blueprint CSRF Integration Demo
          </h1>
          <p className="text-gray-600">
            Demonstrates secure CSRF protection between Next.js frontend and Blueprint Go backend
          </p>
        </header>

        <div className="grid gap-6">
          {/* CSRF Status */}
          <CSRFStatus />

          {/* Instructions */}
          <div className="bg-yellow-50 p-4 rounded-lg border border-yellow-200">
            <h3 className="font-semibold text-yellow-900 mb-2">Setup Instructions</h3>
            <ol className="text-sm text-yellow-800 space-y-1">
              <li>1. Start the Blueprint API server: <code className="bg-yellow-100 px-1 rounded">cd samples/nextjs-api && go run main.go</code></li>
              <li>2. The API server should be running on <a href="http://localhost:8080" target="_blank" className="underline">http://localhost:8080</a></li>
              <li>3. This Next.js app runs on <a href="http://localhost:3000" target="_blank" className="underline">http://localhost:3000</a></li>
              <li>4. Try creating, viewing, and deleting users below to test CSRF protection</li>
            </ol>
          </div>

          {/* User Management Demo */}
          <UserManager />

          {/* Footer Info */}
          <div className="bg-gray-50 p-4 rounded-lg border border-gray-200">
            <h3 className="font-semibold text-gray-900 mb-2">How CSRF Protection Works</h3>
            <ul className="text-sm text-gray-700 space-y-1">
              <li>• GET requests (like loading users) don't require CSRF tokens</li>
              <li>• POST/PUT/DELETE requests require valid CSRF tokens in the X-CSRF-Token header</li>
              <li>• Tokens are tied to your session and refresh after each successful request</li>
              <li>• Requests without valid tokens return 403 Forbidden errors</li>
              <li>• The API client automatically handles token refresh and retry logic</li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
}