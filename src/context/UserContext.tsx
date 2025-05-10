import React, { createContext, useContext, useState, useEffect } from 'react';
import { useAuth } from './AuthContext';
import { fetchCurrentUser } from '../utils/api';

interface User {
    id: string;
    username: string;
    name: string;
    role?: string;
}

interface UserContextType {
    currentUser: User | null;
    users: User[];
    login: () => void;
    logout: () => void;
    switchUser: () => void;
}

const UserContext = createContext<UserContextType | undefined>(undefined);

export const UserProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
    const { currentUser: authUser } = useAuth();
    const [currentUser, setCurrentUser] = useState<User | null>(null);
    const [users, setUsers] = useState<User[]>([]);

    useEffect(() => {
        // If Firebase auth user changes, update our user context
        if (authUser) {
            // Create a base user from Firebase auth user
            const baseUser: User = {
                id: authUser.uid,
                username: authUser.email || '',
                name: authUser.displayName || authUser.email?.split('@')[0] || 'User',
                role: 'user' // Default role
            };
            
            // Store userId in localStorage for other components
            localStorage.setItem('userId', baseUser.id);
            
            // First try to get user info from the backend API
            const fetchUser = async () => {
                try {
                    const apiUser = await fetchCurrentUser();
                    if (apiUser) {
                        console.log('Got user from API:', apiUser);
                        
                        // Create a complete user by merging Firebase and API data
                        const completeUser: User = {
                            ...baseUser,
                            role: apiUser.role || baseUser.role,
                            // Prefer API name if available
                            name: apiUser.name || baseUser.name
                        };
                        
                        console.log('Setting user with API data:', completeUser);
                        setCurrentUser(completeUser);
                        
                        // Store the complete user in localStorage
                        localStorage.setItem('user', JSON.stringify(completeUser));
                        
                        // Update users array
                        setUsers([completeUser]);
                        return;
                    }
                } catch (error) {
                    console.error('Error fetching user from API:', error);
                }
                
                // Fallback to local detection if API fails
                
                // Special check for Patrick Bennett's Firebase UID
                if (authUser.uid === 'UgwzWuP8iHNF8nhqDHMwFFcg8Sc2') {
                    baseUser.role = 'superadmin';
                    console.log('User ID matches Patrick Bennett - setting role to superadmin');
                }
                
                // Check localStorage for existing user data
                const storedUserJSON = localStorage.getItem('user');
                if (storedUserJSON) {
                    try {
                        const storedUser = JSON.parse(storedUserJSON);
                        // If stored user matches current user ID, use its role
                        if (storedUser.id === baseUser.id && storedUser.role) {
                            baseUser.role = storedUser.role;
                            console.log('Using role from localStorage:', baseUser.role);
                        }
                    } catch (error) {
                        console.error('Error parsing stored user data:', error);
                    }
                }
                
                // For development, set admin role based on email
                if (baseUser.username.includes('admin')) {
                    baseUser.role = 'superadmin';
                    console.log('Email contains "admin", setting role to superadmin');
                }
                
                // Set superadmin role if email contains patrick
                if (baseUser.username.toLowerCase().includes('patrick')) {
                    baseUser.role = 'superadmin';
                    console.log('Email contains "patrick", setting role to superadmin');
                }
                
                console.log('Setting user with fallback detection:', baseUser);
                setCurrentUser(baseUser);
                
                // Store the user in localStorage
                localStorage.setItem('user', JSON.stringify(baseUser));
                
                // Update users array
                setUsers([baseUser]);
            };
            
            fetchUser();
        } else {
            setCurrentUser(null);
            localStorage.removeItem('userId');
            localStorage.removeItem('user');
            setUsers([]);
            console.log('User context cleared - user logged out');
        }
    }, [authUser]);

    const login = () => {
        // This is now handled by Firebase Auth
        console.log('Regular login bypassed, using Firebase auth instead');
    };

    const logout = () => {
        // This is now handled in AuthContext
        console.log('Regular logout bypassed, use Firebase auth logout instead');
    };

    const switchUser = () => {
        // This may not be needed with Firebase, but keeping for compatibility
        console.log('User switching not supported with Firebase auth');
    };

    return (
        <UserContext.Provider value={{ currentUser, users, login, logout, switchUser }}>
            {children}
        </UserContext.Provider>
    );
};

export const useUser = () => {
    const context = useContext(UserContext);
    if (context === undefined) {
        throw new Error('useUser must be used within a UserProvider');
    }
    return context;
}; 