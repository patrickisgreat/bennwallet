import { Navigate, Outlet } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import React from 'react';

type ProtectedRouteProps = {
  redirectPath?: string;
  children?: React.ReactNode;
};

export const ProtectedRoute = ({ redirectPath = '/login', children }: ProtectedRouteProps) => {
  const { currentUser, loading } = useAuth();

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-indigo-500"></div>
      </div>
    );
  }

  if (!currentUser) {
    return <Navigate to={redirectPath} replace />;
  }

  return children ? <>{children}</> : <Outlet />;
};

export default ProtectedRoute; 