import { render } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import ProtectedRoute from '../ProtectedRoute';

// Mock the AuthContext with a more complete mock
vi.mock('../../context/AuthContext', () => ({
  useAuth: vi.fn()
}));

// Import the mocked module
import { useAuth } from '../../context/AuthContext';

describe('ProtectedRoute', () => {
  beforeEach(() => {
    // Reset the mock before each test
    vi.mocked(useAuth).mockReset();
  });

  it('renders loading indicator when auth is loading', () => {
    vi.mocked(useAuth).mockReturnValue({
      currentUser: null,
      loading: true,
      error: null,
      signInWithGoogle: vi.fn(),
      signInWithEmail: vi.fn(),
      signUpWithEmail: vi.fn(),
      logout: vi.fn(),
      resetPassword: vi.fn(),
      updateUserProfile: vi.fn(),
      clearError: vi.fn(),
      emailVerificationRequired: vi.fn()
    });

    const { container } = render(
      <MemoryRouter>
        <ProtectedRoute>
          <div>Protected Content</div>
        </ProtectedRoute>
      </MemoryRouter>
    );

    expect(container.querySelector('.animate-spin')).not.toBeNull();
  });

  it('redirects to login when user is not authenticated', () => {
    vi.mocked(useAuth).mockReturnValue({
      currentUser: null,
      loading: false,
      error: null,
      signInWithGoogle: vi.fn(),
      signInWithEmail: vi.fn(),
      signUpWithEmail: vi.fn(),
      logout: vi.fn(),
      resetPassword: vi.fn(),
      updateUserProfile: vi.fn(),
      clearError: vi.fn(),
      emailVerificationRequired: vi.fn()
    });

    const { container } = render(
      <MemoryRouter initialEntries={['/protected']}>
        <Routes>
          <Route path="/login" element={<div data-testid="login-page">Login Page</div>} />
          <Route 
            path="/protected" 
            element={
              <ProtectedRoute>
                <div>Protected Content</div>
              </ProtectedRoute>
            }
          />
        </Routes>
      </MemoryRouter>
    );

    expect(container.querySelector('[data-testid="login-page"]')).not.toBeNull();
  });

  it('renders children when user is authenticated', () => {
    vi.mocked(useAuth).mockReturnValue({
      currentUser: { 
        uid: 'test-uid',
        email: 'test@example.com',
        displayName: 'Test User',
        photoURL: null,
        emailVerified: true
      },
      loading: false,
      error: null,
      signInWithGoogle: vi.fn(),
      signInWithEmail: vi.fn(),
      signUpWithEmail: vi.fn(),
      logout: vi.fn(),
      resetPassword: vi.fn(),
      updateUserProfile: vi.fn(),
      clearError: vi.fn(),
      emailVerificationRequired: vi.fn()
    });

    const { container } = render(
      <MemoryRouter>
        <ProtectedRoute>
          <div data-testid="protected-content">Protected Content</div>
        </ProtectedRoute>
      </MemoryRouter>
    );

    expect(container.querySelector('[data-testid="protected-content"]')).not.toBeNull();
  });
}); 