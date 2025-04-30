import React, { createContext, useContext, useState, useEffect } from 'react';
import { 
  User as FirebaseUser,
  createUserWithEmailAndPassword, 
  signInWithEmailAndPassword, 
  signInWithPopup, 
  signOut, 
  onAuthStateChanged,
  sendPasswordResetEmail,
  sendEmailVerification,
  updateProfile
} from 'firebase/auth';
import { auth, googleProvider } from '../firebase/firebase';

// Define user type
export interface User {
  uid: string;
  email: string | null;
  displayName: string | null;
  photoURL: string | null;
  emailVerified: boolean;
}

// Define context type
interface AuthContextType {
  currentUser: User | null;
  loading: boolean;
  error: string | null;
  signInWithGoogle: () => Promise<void>;
  signInWithEmail: (email: string, password: string) => Promise<void>;
  signUpWithEmail: (email: string, password: string, name: string) => Promise<void>;
  logout: () => Promise<void>;
  resetPassword: (email: string) => Promise<void>;
  updateUserProfile: (data: { displayName?: string; photoURL?: string }) => Promise<void>;
  clearError: () => void;
  emailVerificationRequired: (requiredFor?: string[]) => boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

// Convert Firebase user to our User type
const formatUser = (user: FirebaseUser): User => {
  return {
    uid: user.uid,
    email: user.email,
    displayName: user.displayName,
    photoURL: user.photoURL,
    emailVerified: user.emailVerified
  };
};

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [currentUser, setCurrentUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Clear any error
  const clearError = () => setError(null);

  // Setup auth state listener
  useEffect(() => {
    const unsubscribe = onAuthStateChanged(auth, (user) => {
      setLoading(true);
      if (user) {
        setCurrentUser(formatUser(user));
      } else {
        setCurrentUser(null);
      }
      setLoading(false);
    });

    // Cleanup subscription
    return unsubscribe;
  }, []);

  // Sign in with Google
  const signInWithGoogle = async () => {
    try {
      setError(null);
      setLoading(true);
      await signInWithPopup(auth, googleProvider);
    } catch (err) {
      console.error('Google Sign In Error:', err);
      setError('Failed to sign in with Google');
    } finally {
      setLoading(false);
    }
  };

  // Sign in with email & password
  const signInWithEmail = async (email: string, password: string) => {
    try {
      setError(null);
      setLoading(true);
      await signInWithEmailAndPassword(auth, email, password);
    } catch (err) {
      console.error('Email Sign In Error:', err);
      setError('Failed to sign in. Please check your email and password.');
    } finally {
      setLoading(false);
    }
  };

  // Sign up with email & password
  const signUpWithEmail = async (email: string, password: string, name: string) => {
    try {
      setError(null);
      setLoading(true);
      
      // Create the user
      const userCredential = await createUserWithEmailAndPassword(auth, email, password);
      
      // Update profile with name
      await updateProfile(userCredential.user, {
        displayName: name
      });
      
      // Send verification email
      await sendEmailVerification(userCredential.user);
      
    } catch (err) {
      console.error('Email Sign Up Error:', err);
      setError('Failed to create account. The email may already be in use.');
    } finally {
      setLoading(false);
    }
  };

  // Logout
  const logout = async () => {
    try {
      setError(null);
      await signOut(auth);
    } catch (err) {
      console.error('Sign Out Error:', err);
      setError('Failed to log out');
    }
  };

  // Reset password
  const resetPassword = async (email: string) => {
    try {
      setError(null);
      setLoading(true);
      await sendPasswordResetEmail(auth, email);
    } catch (err) {
      console.error('Password Reset Error:', err);
      setError('Failed to send password reset email');
    } finally {
      setLoading(false);
    }
  };

  // Update user profile
  const updateUserProfile = async (data: { displayName?: string; photoURL?: string }) => {
    try {
      setError(null);
      setLoading(true);
      if (auth.currentUser) {
        await updateProfile(auth.currentUser, data);
        setCurrentUser(formatUser(auth.currentUser));
      }
    } catch (err) {
      console.error('Update Profile Error:', err);
      setError('Failed to update profile');
    } finally {
      setLoading(false);
    }
  };

  // Check if email verification is required
  const emailVerificationRequired = (requiredFor: string[] = []) => {
    // If no user is logged in or user's email is verified, verification is not required
    if (!currentUser || currentUser.emailVerified) {
      return false;
    }

    // If requiredFor array is empty, verification is required for all routes
    if (requiredFor.length === 0) {
      return true;
    }

    // Check if the current path requires verification
    const currentPath = window.location.pathname;
    return requiredFor.some(path => currentPath.startsWith(path));
  };

  const value = {
    currentUser,
    loading,
    error,
    signInWithGoogle,
    signInWithEmail,
    signUpWithEmail,
    logout,
    resetPassword,
    updateUserProfile,
    clearError,
    emailVerificationRequired
  };

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
};

// Custom hook to use auth context
export const useAuth = () => {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};

export default AuthContext; 