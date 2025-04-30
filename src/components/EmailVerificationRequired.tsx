import { Navigate, Outlet } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import React from 'react';

interface EmailVerificationRequiredProps {
  children?: React.ReactNode;
}

const EmailVerificationRequired: React.FC<EmailVerificationRequiredProps> = ({ children }) => {
  const { currentUser } = useAuth();

  // If there's no user or the email is verified, render the children/outlet
  if (!currentUser || currentUser.emailVerified) {
    return children ? <>{children}</> : <Outlet />;
  }

  // If email is not verified, redirect to the verification page
  return <Navigate to="/verify-email" replace />;
};

export default EmailVerificationRequired; 