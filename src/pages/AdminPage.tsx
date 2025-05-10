import React, { useState, useEffect } from 'react';
import { fetchUsers, User, fetchUserPermissions, Permission, grantPermission, revokePermission, fetchAllPermissions } from '../utils/api';
import { useAuth } from '../context/AuthContext';

const RESOURCE_TYPES = [
  { value: 'transactions', label: 'Transactions' },
  { value: 'categories', label: 'Categories' },
  { value: 'ynab_config', label: 'YNAB Configuration' }
];

const PERMISSION_TYPES = [
  { value: 'read', label: 'View Only' },
  { value: 'write', label: 'Edit' }
];

function AdminPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const { currentUser } = useAuth();
  
  // Permission management states
  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [selectedUser, setSelectedUser] = useState<string>('');
  const [selectedTargetUser, setSelectedTargetUser] = useState<string>('');
  const [selectedResourceType, setSelectedResourceType] = useState<string>('transactions');
  const [selectedPermissionType, setSelectedPermissionType] = useState<string>('read');
  const [expiryDate, setExpiryDate] = useState<string>('');
  const [permissionsLoading, setPermissionsLoading] = useState(false);

  const [isSuperAdmin, setIsSuperAdmin] = useState(false);
  
  // Add state for all permissions
  const [allPermissions, setAllPermissions] = useState<Permission[]>([]);
  const [allPermissionsLoading, setAllPermissionsLoading] = useState(false);

  useEffect(() => {
    const loadUsers = async () => {
      setLoading(true);
      try {
        // Try to fetch users from the API
        const fetchedUsers = await fetchUsers();
        
        // Set the users from the API response, even if empty
        setUsers(fetchedUsers || []);
        setError(null);
      } catch (err) {
        console.error('Error loading users:', err);
        setError('Failed to load users. Please verify the backend is running properly.');
        setUsers([]);
      } finally {
        setLoading(false);
      }
    };

    loadUsers();
  }, []);

  // Load permissions when a user is selected
  useEffect(() => {
    if (selectedUser) {
      loadUserPermissions(selectedUser);
    }
  }, [selectedUser]);

  useEffect(() => {
    // Check if the user is a superadmin
    if (currentUser) {
      const user = currentUser as unknown as { role?: string };
      setIsSuperAdmin(user.role === 'superadmin');
      
      // Also check localStorage as fallback
      const userJSON = localStorage.getItem('user');
      if (userJSON) {
        try {
          const userData = JSON.parse(userJSON);
          if (userData.role === 'superadmin') {
            setIsSuperAdmin(true);
          }
        } catch (e) {
          console.error('Error parsing user from localStorage:', e);
        }
      }
    }
  }, [currentUser]);

  const loadUserPermissions = async (userId: string) => {
    setPermissionsLoading(true);
    try {
      // Fetch all permissions related to this user
      const allPermissions = await fetchUserPermissions(userId);
      
      // We want to see both permissions granted to this user and permissions they've granted to others
      const relevantPermissions = allPermissions.filter(p => 
        p.ownerUserId === userId || p.grantedUserId === userId
      );
      
      setPermissions(relevantPermissions);
      console.log("Loaded permissions for user:", userId, relevantPermissions);
    } catch (err) {
      console.error('Error loading permissions:', err);
      setError('Failed to load user permissions.');
    } finally {
      setPermissionsLoading(false);
    }
  };

  // Function to determine if the current user can modify a given user's role
  const canEditUserRole = (targetUser: User) => {
    if (!currentUser) return false;
    
    // Get current user's role from localStorage if available
    let currentUserRole = 'user';
    const userJSON = localStorage.getItem('user');
    if (userJSON) {
      try {
        const userData = JSON.parse(userJSON);
        if (userData.role) {
          currentUserRole = userData.role;
        }
      } catch (e) {
        console.error('Error parsing user from localStorage:', e);
      }
    }
    
    // Get target user's role
    const targetUserRole = targetUser.role || 'user';
    
    // Superadmins can edit anyone
    if (currentUserRole === 'superadmin') return true;
    
    // Admins can edit regular users but not other admins or superadmins
    if (currentUserRole === 'admin') {
      return targetUserRole !== 'admin' && targetUserRole !== 'superadmin';
    }
    
    // Regular users can't edit anyone
    return false;
  };

  const handleRoleChange = (userId: string, newRole: string) => {
    // Modify the user in the local state
    const updatedUsers = users.map(user => {
      if (user.id === userId) {
        return { ...user, role: newRole };
      }
      return user;
    });
    
    setUsers(updatedUsers);
    
    // Also update localStorage if this is the current user
    const userJSON = localStorage.getItem('user');
    if (userJSON) {
      try {
        const userData = JSON.parse(userJSON);
        if (userData.id === userId) {
          userData.role = newRole;
          localStorage.setItem('user', JSON.stringify(userData));
          alert(`Your role has been updated to ${newRole}. You may need to refresh the page for changes to take effect.`);
        } else {
          console.log(`Updated role for user ${userId} to ${newRole}`);
          alert(`User role updated to ${newRole}`);
        }
      } catch (e) {
        console.error('Error updating user role in localStorage:', e);
      }
    }
  };

  // Check if current user is admin
  const isAdmin = () => {
    if (!currentUser) {
      console.log("AdminPage - No current user");
      return false;
    }
    
    // First check context
    const user = currentUser as unknown as { role?: string };
    if (user.role === 'admin' || user.role === 'superadmin') {
      console.log("AdminPage - User has admin role from context:", user.role);
      return true;
    }
    
    // Fallback to localStorage check
    const userJSON = localStorage.getItem('user');
    if (userJSON) {
      try {
        const userData = JSON.parse(userJSON);
        const hasAdminRole = userData.role === 'admin' || userData.role === 'superadmin';
        console.log("AdminPage - User role from localStorage:", userData.role, "isAdmin:", hasAdminRole);
        return hasAdminRole;
      } catch (e) {
        console.error('Error parsing user from localStorage:', e);
      }
    } else {
      console.log("AdminPage - No user data in localStorage");
    }
    
    // Default to false if all checks fail
    return false;
  };

  // Handle granting a new permission
  const handleGrantPermission = async () => {
    if (!selectedTargetUser || !selectedResourceType || !selectedPermissionType) {
      setError('Please select a user, resource type, and permission type.');
      return;
    }

    try {
      const expiry = expiryDate ? new Date(expiryDate) : undefined;
      const success = await grantPermission(
        selectedTargetUser,
        selectedResourceType,
        selectedPermissionType,
        expiry
      );

      if (success) {
        // Reload permissions to show the new one
        await loadUserPermissions(selectedUser);
        alert('Permission granted successfully!');
      } else {
        setError('Failed to grant permission.');
      }
    } catch (err) {
      console.error('Error granting permission:', err);
      setError('An error occurred while granting permission.');
    }
  };

  // Handle revoking a permission
  const handleRevokePermission = async (permission: Permission) => {
    if (!confirm('Are you sure you want to revoke this permission?')) {
      return;
    }

    try {
      const success = await revokePermission(
        permission.grantedUserId,
        permission.ownerUserId,
        permission.resourceType,
        permission.permissionType
      );

      if (success) {
        // Reload permissions to show the change
        await loadUserPermissions(selectedUser);
        alert('Permission revoked successfully!');
      } else {
        setError('Failed to revoke permission.');
      }
    } catch (err) {
      console.error('Error revoking permission:', err);
      setError('An error occurred while revoking permission.');
    }
  };

  // Get user name by ID
  const getUserName = (userId: string): string => {
    const user = users.find(u => u.id === userId);
    return user ? user.name || user.username || userId : userId;
  };

  // Format permission type for display
  const formatPermissionType = (type: string): string => {
    return type === 'read' ? 'View Only' : type === 'write' ? 'Edit' : type;
  };

  // Format resource type for display
  const formatResourceType = (type: string): string => {
    switch (type) {
      case 'transactions': return 'Transactions';
      case 'categories': return 'Categories';
      case 'ynab_config': return 'YNAB Configuration';
      default: return type;
    }
  };

  // Load all permissions (superadmin only)
  const loadAllPermissions = async () => {
    if (!isSuperAdmin) {
      setError('Only superadmins can view all permissions');
      return;
    }
    
    setAllPermissionsLoading(true);
    try {
      const permissions = await fetchAllPermissions();
      setAllPermissions(permissions);
      console.log('Loaded all permissions:', permissions);
    } catch (err) {
      console.error('Error loading all permissions:', err);
      setError('Failed to load all permissions');
    } finally {
      setAllPermissionsLoading(false);
    }
  };

  return (
    <div className="container mx-auto p-4">
      <h1 className="text-2xl font-bold mb-4">Admin Panel</h1>

      {error && (
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
          {error}
          <button className="float-right font-bold" onClick={() => setError(null)}>
            &times;
          </button>
        </div>
      )}

      {!isAdmin() ? (
        <div className="bg-yellow-100 border border-yellow-400 text-yellow-700 px-4 py-3 rounded mb-4">
          You do not have admin privileges to access this page.
        </div>
      ) : (
        <>
          <section className="mb-6">
            <h2 className="text-xl font-semibold mb-3">User Management</h2>
            {loading ? (
              <div className="text-center py-4">Loading users...</div>
            ) : users.length === 0 ? (
              <div className="bg-gray-100 p-4 rounded text-center">
                <p className="text-gray-600">No users found. Please check your backend configuration.</p>
              </div>
            ) : (
              <div className="bg-white shadow rounded-lg overflow-hidden">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                        Name
                      </th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                        Email
                      </th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                        Role
                      </th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                        Actions
                      </th>
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {users.map((user) => (
                      <tr key={user.id}>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="text-sm font-medium text-gray-900">{user.name}</div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="text-sm text-gray-500">{user.email}</div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <span className={`px-2 inline-flex text-xs leading-5 font-semibold rounded-full 
                            ${user.role === 'superadmin' ? 'bg-purple-100 text-purple-800' : 
                              user.role === 'admin' ? 'bg-blue-100 text-blue-800' : 
                              'bg-green-100 text-green-800'}`}>
                            {user.role || 'user'}
                          </span>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                          {canEditUserRole(user) ? (
                            <div className="flex space-x-2">
                              <button 
                                className="text-indigo-600 hover:text-indigo-900"
                                onClick={() => handleRoleChange(user.id, 'admin')}
                              >
                                Make Admin
                              </button>
                            </div>
                          ) : (
                            <span className="text-gray-400">No actions available</span>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>

          {/* Permission Management Section */}
          <section className="mb-6">
            <h2 className="text-xl font-semibold mb-3">Permission Management</h2>
            <div className="bg-white shadow rounded-lg p-6">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div>
                  <h3 className="font-medium text-gray-700 mb-4">Configure User Permissions</h3>
                  
                  <div className="mb-4">
                    <label className="block text-sm font-medium text-gray-700 mb-1">
                      Select Data Owner
                    </label>
                    <select 
                      className="w-full border-gray-300 rounded-md shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
                      value={selectedUser}
                      onChange={(e) => setSelectedUser(e.target.value)}
                    >
                      <option value="">-- Select User --</option>
                      {users.map(user => (
                        <option key={user.id} value={user.id}>
                          {user.name || user.username}
                        </option>
                      ))}
                    </select>
                  </div>

                  {selectedUser && (
                    <div className="border rounded-md p-4 bg-gray-50">
                      <h4 className="font-medium mb-2">Grant New Permission</h4>
                      
                      <div className="mb-3">
                        <label className="block text-sm font-medium text-gray-700 mb-1">
                          Grant Access To
                        </label>
                        <select 
                          className="w-full border-gray-300 rounded-md shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
                          value={selectedTargetUser}
                          onChange={(e) => setSelectedTargetUser(e.target.value)}
                        >
                          <option value="">-- Select User --</option>
                          {users
                            .filter(user => user.id !== selectedUser) // Don't show the owner
                            .map(user => (
                              <option key={user.id} value={user.id}>
                                {user.name || user.username}
                              </option>
                            ))
                          }
                        </select>
                      </div>
                      
                      <div className="mb-3">
                        <label className="block text-sm font-medium text-gray-700 mb-1">
                          Resource Type
                        </label>
                        <select 
                          className="w-full border-gray-300 rounded-md shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
                          value={selectedResourceType}
                          onChange={(e) => setSelectedResourceType(e.target.value)}
                        >
                          {RESOURCE_TYPES.map(type => (
                            <option key={type.value} value={type.value}>
                              {type.label}
                            </option>
                          ))}
                        </select>
                      </div>
                      
                      <div className="mb-3">
                        <label className="block text-sm font-medium text-gray-700 mb-1">
                          Permission Type
                        </label>
                        <select 
                          className="w-full border-gray-300 rounded-md shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
                          value={selectedPermissionType}
                          onChange={(e) => setSelectedPermissionType(e.target.value)}
                        >
                          {PERMISSION_TYPES.map(type => (
                            <option key={type.value} value={type.value}>
                              {type.label}
                            </option>
                          ))}
                        </select>
                      </div>
                      
                      <div className="mb-4">
                        <label className="block text-sm font-medium text-gray-700 mb-1">
                          Expiry Date (Optional)
                        </label>
                        <input 
                          type="date" 
                          className="w-full border-gray-300 rounded-md shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
                          value={expiryDate}
                          onChange={(e) => setExpiryDate(e.target.value)}
                        />
                      </div>
                      
                      <button
                        className="w-full bg-indigo-600 text-white py-2 px-4 rounded-md hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
                        onClick={handleGrantPermission}
                      >
                        Grant Permission
                      </button>
                    </div>
                  )}
                </div>
                
                <div>
                  {selectedUser && (
                    <>
                      <h3 className="font-medium text-gray-700 mb-4">
                        Current Permissions for {getUserName(selectedUser)}
                      </h3>
                      
                      {permissionsLoading ? (
                        <div className="text-center py-4">Loading permissions...</div>
                      ) : permissions.length === 0 ? (
                        <div className="text-center py-4 text-gray-500">No permissions found.</div>
                      ) : (
                        <div className="overflow-hidden border rounded-md">
                          <table className="min-w-full divide-y divide-gray-200">
                            <thead className="bg-gray-50">
                              <tr>
                                <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                                  Data Owner
                                </th>
                                <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                                  Granted To
                                </th>
                                <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                                  Resource
                                </th>
                                <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                                  Access
                                </th>
                                <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                                  Actions
                                </th>
                              </tr>
                            </thead>
                            <tbody className="bg-white divide-y divide-gray-200">
                              {permissions.map((permission) => (
                                <tr key={permission.id}>
                                  <td className="px-4 py-2 whitespace-nowrap text-sm">
                                    {getUserName(permission.ownerUserId)}
                                  </td>
                                  <td className="px-4 py-2 whitespace-nowrap text-sm">
                                    {getUserName(permission.grantedUserId)}
                                  </td>
                                  <td className="px-4 py-2 whitespace-nowrap text-sm">
                                    {formatResourceType(permission.resourceType)}
                                  </td>
                                  <td className="px-4 py-2 whitespace-nowrap text-sm">
                                    {formatPermissionType(permission.permissionType)}
                                  </td>
                                  <td className="px-4 py-2 whitespace-nowrap text-sm">
                                    <button 
                                      className="text-red-600 hover:text-red-900"
                                      onClick={() => handleRevokePermission(permission)}
                                    >
                                      Revoke
                                    </button>
                                  </td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        </div>
                      )}
                    </>
                  )}
                </div>
              </div>
            </div>
          </section>

          {/* Add superadmin section for viewing all permissions */}
          {isSuperAdmin && (
            <section className="mb-6">
              <h2 className="text-xl font-semibold mb-3">System-wide Permissions</h2>
              <button 
                className="w-full bg-indigo-600 text-white py-2 px-4 rounded-md hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
                onClick={loadAllPermissions}
                disabled={allPermissionsLoading}
              >
                {allPermissionsLoading ? 'Loading...' : 'View All Permissions'}
              </button>
              
              {allPermissions.length > 0 && (
                <div className="mt-4">
                  <h3 className="font-medium text-gray-700 mb-4">All Permissions</h3>
                  <div className="overflow-hidden border rounded-md">
                    <table className="min-w-full divide-y divide-gray-200">
                      <thead className="bg-gray-50">
                        <tr>
                          <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                            Data Owner
                          </th>
                          <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                            Granted To
                          </th>
                          <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                            Resource
                          </th>
                          <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                            Access
                          </th>
                          <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                            Created
                          </th>
                          <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                            Expires
                          </th>
                        </tr>
                      </thead>
                      <tbody className="bg-white divide-y divide-gray-200">
                        {allPermissions.map((permission) => (
                          <tr key={permission.id}>
                            <td className="px-4 py-2 whitespace-nowrap text-sm">
                              {getUserName(permission.ownerUserId)}
                            </td>
                            <td className="px-4 py-2 whitespace-nowrap text-sm">
                              {getUserName(permission.grantedUserId)}
                            </td>
                            <td className="px-4 py-2 whitespace-nowrap text-sm">
                              {formatResourceType(permission.resourceType)}
                            </td>
                            <td className="px-4 py-2 whitespace-nowrap text-sm">
                              {formatPermissionType(permission.permissionType)}
                            </td>
                            <td className="px-4 py-2 whitespace-nowrap text-sm">
                              {new Date(permission.createdAt).toLocaleDateString()}
                            </td>
                            <td className="px-4 py-2 whitespace-nowrap text-sm">
                              {permission.expiresAt ? new Date(permission.expiresAt).toLocaleDateString() : 'Never'}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}
            </section>
          )}
        </>
      )}
    </div>
  );
}

export default AdminPage; 