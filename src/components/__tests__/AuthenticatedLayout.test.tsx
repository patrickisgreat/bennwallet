import { render } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { vi, describe, it, expect } from 'vitest';
import AuthenticatedLayout from '../AuthenticatedLayout';

// Mock the dependencies
vi.mock('../../context/AuthContext', () => ({
  useAuth: vi.fn()
}));

vi.mock('../MainNavigation', () => ({
  default: () => <div data-testid="main-navigation">Main Navigation</div>
}));

// Mock the Outlet component from react-router-dom
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    Outlet: () => <div data-testid="outlet-content">Outlet Content</div>
  };
});

import { useAuth } from '../../context/AuthContext';

describe('AuthenticatedLayout', () => {
  it('renders loading state when loading', () => {
    vi.mocked(useAuth).mockReturnValue({
      loading: true,
      currentUser: null,
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
        <AuthenticatedLayout />
      </MemoryRouter>
    );

    expect(container.querySelector('.animate-spin')).not.toBeNull();
  });

  it('renders navigation and content when not loading', () => {
    vi.mocked(useAuth).mockReturnValue({
      loading: false,
      currentUser: {
        uid: 'test-uid',
        email: 'test@example.com',
        displayName: 'Test User',
        photoURL: null,
        emailVerified: true
      },
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
        <AuthenticatedLayout />
      </MemoryRouter>
    );

    expect(container.querySelector('[data-testid="main-navigation"]')).not.toBeNull();
    expect(container.querySelector('[data-testid="outlet-content"]')).not.toBeNull();
  });
}); 