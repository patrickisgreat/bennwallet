import { vi } from 'vitest';

// Mock Firebase app
const mockApp = {
  name: 'mock-app',
  options: {},
};

// Mock Analytics
const mockAnalytics = {
  logEvent: vi.fn(),
  setCurrentScreen: vi.fn(),
  setUserId: vi.fn(),
};

// Mock Auth
const mockAuth = {
  currentUser: null,
  onAuthStateChanged: vi.fn(),
  signInWithEmailAndPassword: vi.fn(),
  signInWithPopup: vi.fn(),
  signOut: vi.fn(),
};

// Mock Google Provider
const mockGoogleProvider = {
  addScope: vi.fn(),
  setCustomParameters: vi.fn(),
};

export const initializeApp = vi.fn().mockReturnValue(mockApp);
export const getAnalytics = vi.fn().mockReturnValue(mockAnalytics);
export const getAuth = vi.fn().mockReturnValue(mockAuth);
export const GoogleAuthProvider = vi.fn().mockImplementation(() => mockGoogleProvider);

export const analytics = mockAnalytics;
export const auth = mockAuth;
export const googleProvider = mockGoogleProvider;

export default mockApp;
