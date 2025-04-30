import { render } from '@testing-library/react';
import { UserProvider, useUser } from '../UserContext';
import { vi, describe, it, expect } from 'vitest';

// Mock the AuthContext
const mockAuthUser = {
  uid: 'test-uid',
  email: 'test@example.com',
  displayName: 'Test User'
};

type MockUser = typeof mockAuthUser | null;
let mockCurrentUser: MockUser = mockAuthUser;

vi.mock('../AuthContext', () => ({
  useAuth: () => ({
    currentUser: mockCurrentUser,
    loading: false,
    error: null
  })
}));

// Test component that uses the UserContext
const TestComponent = () => {
  const { currentUser } = useUser();
  return (
    <div>
      {currentUser ? (
        <div data-testid="user-info">
          <div data-testid="user-id">{currentUser.id}</div>
          <div data-testid="user-name">{currentUser.name}</div>
          <div data-testid="user-username">{currentUser.username}</div>
        </div>
      ) : (
        <div data-testid="no-user">No user</div>
      )}
    </div>
  );
};

describe('UserContext', () => {
  it('provides user information from Firebase auth', async () => {
    mockCurrentUser = mockAuthUser;

    const { findByTestId } = render(
      <UserProvider>
        <TestComponent />
      </UserProvider>
    );

    // Wait for the user info to be rendered
    const userInfo = await findByTestId('user-info');
    const userId = await findByTestId('user-id');
    const userName = await findByTestId('user-name');
    const userUsername = await findByTestId('user-username');

    expect(userInfo).toBeInTheDocument();
    expect(userId).toHaveTextContent('test-uid');
    expect(userName).toHaveTextContent('Test User');
    expect(userUsername).toHaveTextContent('test@example.com');
  });

  it('handles no user state', async () => {
    // Mock auth with no user
    mockCurrentUser = null;

    const { findByTestId } = render(
      <UserProvider>
        <TestComponent />
      </UserProvider>
    );

    // Wait for the no-user element to be rendered
    const noUser = await findByTestId('no-user');
    expect(noUser).toBeInTheDocument();
  });
}); 