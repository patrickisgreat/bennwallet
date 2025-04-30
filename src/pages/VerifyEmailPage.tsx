import { useState } from 'react';
import { Link } from 'react-router-dom';
import { sendEmailVerification } from 'firebase/auth';
import { auth } from '../firebase/firebase';
import { useAuth } from '../context/AuthContext';

function VerifyEmailPage() {
  const { currentUser } = useAuth();
  const [message, setMessage] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  
  const handleResendVerification = async () => {
    if (!auth.currentUser) return;
    
    try {
      setLoading(true);
      setError('');
      await sendEmailVerification(auth.currentUser);
      setMessage('Verification email sent! Check your inbox.');
    } catch (err) {
      setError('Failed to send verification email. Please try again later.');
      console.error('Error sending verification email:', err);
    } finally {
      setLoading(false);
    }
  };
  
  if (!currentUser) {
    return <div className="text-center py-10">Loading...</div>;
  }
  
  // If email is already verified, no need to show this page
  if (currentUser.emailVerified) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
        <div className="max-w-md w-full space-y-8 bg-white p-8 rounded-lg shadow">
          <div>
            <h2 className="mt-6 text-center text-3xl font-extrabold text-gray-900">
              Email Verified
            </h2>
            <p className="mt-2 text-center text-sm text-gray-600">
              Your email has been successfully verified.
            </p>
          </div>
          <div className="mt-6">
            <Link 
              to="/" 
              className="w-full flex justify-center py-2 px-4 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
            >
              Go to Dashboard
            </Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-md w-full space-y-8 bg-white p-8 rounded-lg shadow">
        <div>
          <h2 className="mt-6 text-center text-3xl font-extrabold text-gray-900">
            Verify Your Email
          </h2>
          <p className="mt-2 text-center text-sm text-gray-600">
            A verification email has been sent to <span className="font-medium">{currentUser.email}</span>.
            Please check your inbox and click the verification link.
          </p>
        </div>
        
        {error && (
          <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded">
            {error}
          </div>
        )}
        
        {message && (
          <div className="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded">
            {message}
          </div>
        )}
        
        <div className="mt-6">
          <button
            onClick={handleResendVerification}
            disabled={loading}
            className="w-full flex justify-center py-2 px-4 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 disabled:bg-indigo-400"
          >
            {loading ? 'Sending...' : 'Resend Verification Email'}
          </button>
          
          <div className="mt-4 text-center">
            <Link 
              to="/" 
              className="font-medium text-indigo-600 hover:text-indigo-500"
            >
              Back to Dashboard
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}

export default VerifyEmailPage; 