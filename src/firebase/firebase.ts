import { initializeApp } from 'firebase/app';
import { getAuth, GoogleAuthProvider } from 'firebase/auth';
import { getAnalytics } from 'firebase/analytics';

// Your Firebase configuration
// Replace with your actual Firebase project config from:
// Firebase console -> Project settings -> General -> Your apps -> Firebase SDK snippet -> Config

/**
 * SETUP INSTRUCTIONS:
 * 1. Go to https://console.firebase.google.com/
 * 2. Create a new project or use an existing one
 * 3. Add a web app to your project
 * 4. Copy the Firebase configuration object and replace the one below
 * 5. Enable Authentication in Firebase console -> Authentication -> Sign-in methods
 * 6. Enable Google Sign-in and Email/Password authentication methods
 */

// Replace the object below with your actual Firebase configuration
const firebaseConfig = {
  apiKey: "AIzaSyBQADPrQxlodCm8pJh9U1Uf4tfInoH25Fc",
  authDomain: "benwallett-ab39d.firebaseapp.com",
  projectId: "benwallett-ab39d",
  storageBucket: "benwallett-ab39d.firebasestorage.app",
  messagingSenderId: "840662435873",
  appId: "1:840662435873:web:d73b002523d86faf77fb60",
  measurementId: "G-JV8X2HJ8CZ"
};

// Initialize Firebase
const app = initializeApp(firebaseConfig);

// Initialize Analytics
export const analytics = getAnalytics(app);

// Authentication
export const auth = getAuth(app);

// Google provider with custom scopes
export const googleProvider = new GoogleAuthProvider();
googleProvider.addScope('https://www.googleapis.com/auth/userinfo.email');
googleProvider.addScope('https://www.googleapis.com/auth/userinfo.profile');

// Set custom parameters for the Google sign-in flow
googleProvider.setCustomParameters({
  prompt: 'select_account'
});

export default app; 