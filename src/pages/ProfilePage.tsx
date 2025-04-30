import { useState, useEffect } from 'react';
import { useAuth } from '../context/AuthContext';

function ProfilePage() {
  const { currentUser, updateUserProfile, loading, error, clearError } = useAuth();
  const [displayName, setDisplayName] = useState('');
  const [photoURL, setPhotoURL] = useState('');
  const [message, setMessage] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    if (currentUser) {
      setDisplayName(currentUser.displayName || '');
      setPhotoURL(currentUser.photoURL || '');
    }
  }, [currentUser]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!currentUser) return;
    
    try {
      setIsSubmitting(true);
      setMessage('');
      clearError();
      
      // Only update fields that have changed
      const updateData: { displayName?: string; photoURL?: string } = {};
      
      if (displayName !== currentUser.displayName) {
        updateData.displayName = displayName;
      }
      
      if (photoURL !== currentUser.photoURL) {
        updateData.photoURL = photoURL;
      }
      
      if (Object.keys(updateData).length > 0) {
        await updateUserProfile(updateData);
        setMessage('Profile updated successfully');
      } else {
        setMessage('No changes to save');
      }
    } catch (err) {
      console.error('Error updating profile:', err);
      // Error is handled by auth context
    } finally {
      setIsSubmitting(false);
    }
  };

  if (!currentUser) {
    return <div className="text-center py-10">Loading...</div>;
  }

  return (
    <div className="min-h-screen bg-gray-100">
      <div className="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
        <div className="px-4 py-6 sm:px-0">
          <div className="bg-white shadow rounded-lg p-6">
            <h1 className="text-2xl font-bold text-gray-900 mb-6">Profile Settings</h1>
            
            {error && (
              <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
                {error}
                <button 
                  onClick={clearError}
                  className="float-right font-bold"
                >
                  &times;
                </button>
              </div>
            )}
            
            {message && (
              <div className="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded mb-4">
                {message}
                <button 
                  onClick={() => setMessage('')}
                  className="float-right font-bold"
                >
                  &times;
                </button>
              </div>
            )}

            <div className="flex flex-col md:flex-row gap-6 mb-6">
              <div className="flex-1">
                <div className="flex items-center justify-center md:justify-start">
                  {photoURL ? (
                    <img 
                      src={photoURL} 
                      alt={displayName || 'User'} 
                      className="h-24 w-24 rounded-full object-cover"
                    />
                  ) : (
                    <div className="h-24 w-24 rounded-full bg-indigo-600 flex items-center justify-center">
                      <span className="text-2xl text-white font-medium">
                        {displayName?.charAt(0).toUpperCase() || 'U'}
                      </span>
                    </div>
                  )}
                </div>
              </div>
              
              <div className="flex-1">
                <div className="mb-4">
                  <p className="text-sm font-medium text-gray-500">Email</p>
                  <p className="text-lg text-gray-900">{currentUser.email}</p>
                </div>
                
                <div className="mb-4">
                  <p className="text-sm font-medium text-gray-500">Account ID</p>
                  <p className="text-sm text-gray-500">{currentUser.uid}</p>
                </div>
                
                <div>
                  <p className="text-sm font-medium text-gray-500">Email Verification</p>
                  <p className={`text-sm ${currentUser.emailVerified ? 'text-green-500' : 'text-yellow-500'}`}>
                    {currentUser.emailVerified ? 'Verified' : 'Not verified'}
                  </p>
                </div>
              </div>
            </div>

            <form onSubmit={handleSubmit}>
              <div className="mb-4">
                <label htmlFor="displayName" className="block text-sm font-medium text-gray-700">
                  Display Name
                </label>
                <input
                  id="displayName"
                  name="displayName"
                  type="text"
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
                />
              </div>
              
              <div className="mb-6">
                <label htmlFor="photoURL" className="block text-sm font-medium text-gray-700">
                  Profile Photo URL
                </label>
                <input
                  id="photoURL"
                  name="photoURL"
                  type="url"
                  value={photoURL}
                  onChange={(e) => setPhotoURL(e.target.value)}
                  placeholder="https://example.com/photo.jpg"
                  className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
                />
                <p className="mt-1 text-xs text-gray-500">
                  Enter a URL to an image file (JPG, PNG, etc.)
                </p>
              </div>
              
              <div className="flex justify-end">
                <button
                  type="submit"
                  disabled={loading || isSubmitting}
                  className="inline-flex justify-center py-2 px-4 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 disabled:bg-indigo-400"
                >
                  {isSubmitting ? 'Saving...' : 'Save Changes'}
                </button>
              </div>
            </form>
          </div>
        </div>
      </div>
    </div>
  );
}

export default ProfilePage; 