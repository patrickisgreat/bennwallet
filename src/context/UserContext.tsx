import React, { createContext, useContext, useState, useEffect } from 'react';
import { useAuth } from './AuthContext';

interface User {
    id: string;
    username: string;
    name: string;
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
            // Create a user from Firebase auth user
            const user: User = {
                id: authUser.uid,
                username: authUser.email || '',
                name: authUser.displayName || authUser.email?.split('@')[0] || 'User'
            };
            setCurrentUser(user);
            localStorage.setItem('userId', user.id);
        } else {
            setCurrentUser(null);
            localStorage.removeItem('userId');
        }
    }, [authUser]);

    useEffect(() => {
        const fetchUsers = async () => {
            try {
                // We may not need to fetch all users for the app
                // But keeping the structure for now
                if (authUser) {
                    console.log('Getting current user info...');
                    setUsers([{
                        id: authUser.uid,
                        username: authUser.email || '',
                        name: authUser.displayName || authUser.email?.split('@')[0] || 'User'
                    }]);
                }
            } catch (error) {
                console.error('Error getting user info:', error);
            }
        };

        fetchUsers();
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