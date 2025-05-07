import React, { useState, useEffect } from 'react';
import { useAuth } from '../context/AuthContext';
import { User, fetchUsers, api } from '../utils/api';

// Define permission types that can be granted
const RESOURCE_TYPES = [
  'transactions',
  'categories',
  'ynab_config',
  'reports'
];

const PERMISSION_TYPES = [
  'read',
  'write'
];

const ROLES = [
  'user',
  'admin',
  'superadmin'
];

// Interface for permission object
interface Permission {
  id: string;
  grantedUserID: string;
  ownerUserID: string;
  resourceType: string;
  permissionType: string;
  createdAt: string;
  expiresAt?: string;
}

// Response type for user role request
interface UserRoleResponse {
  userId: string;
  role: string;
}

function AdminPage() {
  const { currentUser } = useAuth();
  // We don't need users from context as we're loading them directly
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [userRole, setUserRole] = useState<string>('');
  const [allUsers, setAllUsers] = useState<User[]>([]);
  const [selectedUser, setSelectedUser] = useState<string>('');
  const [userPermissions, setUserPermissions] = useState<Permission[]>([]);
  
  // New permission form state
  const [newPermission, setNewPermission] = useState({
    granteeId: '',
    resourceType: RESOURCE_TYPES[0],
    permissionType: PERMISSION_TYPES[0],
    expiresAt: '' // Optional expiry date
  });

  // New role form state
  const [roleChange, setRoleChange] = useState({
    userId: '',
    newRole: ROLES[0]
  });

  useEffect(() => {
    // Load the current user's role
    const loadUserRole = async () => {
      try {
        if (currentUser) {
          const response = await api.get<UserRoleResponse>(`/users/${currentUser.uid}/role`);
          if (response.data && response.data.role) {
            setUserRole(response.data.role);
          } else {
            setError('Could not determine user role');
          }
        }
      } catch (err) {
        console.error('Error fetching user role:', err);
        setError('Failed to load user role');
      }
    };

    // Load all users
    const loadUsers = async () => {
      try {
        if (currentUser) {
          const fetchedUsers = await fetchUsers();
          setAllUsers(fetchedUsers);
        }
      } catch (err) {
        console.error('Error fetching users:', err);
        setError('Failed to load users');
      }
    };

    setLoading(true);
    Promise.all([loadUserRole(), loadUsers()])
      .finally(() => setLoading(false));
  }, [currentUser]);

  // Load a specific user's permissions
  const loadUserPermissions = async (userId: string) => {
    if (!userId) return;
    
    setLoading(true);
    try {
      const response = await api.get<Permission[]>(`/permissions?userId=${userId}`);
      setUserPermissions(response.data || []);
      setSelectedUser(userId);
    } catch (err) {
      console.error('Error fetching user permissions:', err);
      setError('Failed to load user permissions');
    } finally {
      setLoading(false);
    }
  };

  // Handle granting a new permission
  const handleGrantPermission = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!currentUser) return;

    setLoading(true);
    setError(null);
    setSuccess(null);

    try {
      await api.post('/permissions', newPermission);
      setSuccess('Permission granted successfully');
      
      // Refresh permissions if we're viewing the user we just granted permission to
      if (selectedUser === newPermission.granteeId) {
        await loadUserPermissions(selectedUser);
      }
      
      // Reset form
      setNewPermission({
        granteeId: '',
        resourceType: RESOURCE_TYPES[0],
        permissionType: PERMISSION_TYPES[0],
        expiresAt: ''
      });
    } catch (err) {
      console.error('Error granting permission:', err);
      setError('Failed to grant permission');
    } finally {
      setLoading(false);
    }
  };

  // Handle revoking a permission
  const handleRevokePermission = async (permission: Permission) => {
    if (!currentUser || !window.confirm('Are you sure you want to revoke this permission?')) {
      return;
    }

    setLoading(true);
    setError(null);
    setSuccess(null);

    try {
      await api.post('/permissions/revoke', {
        granteeId: permission.grantedUserID,
        ownerId: permission.ownerUserID,
        resourceType: permission.resourceType,
        permissionType: permission.permissionType
      });
      
      setSuccess('Permission revoked successfully');
      
      // Refresh the permissions list
      await loadUserPermissions(selectedUser);
    } catch (err) {
      console.error('Error revoking permission:', err);
      setError('Failed to revoke permission');
    } finally {
      setLoading(false);
    }
  };

  // Handle changing a user's role
  const handleRoleChange = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!currentUser) return;

    setLoading(true);
    setError(null);
    setSuccess(null);

    try {
      await api.post('/users/role', roleChange);
      setSuccess(`User role updated to ${roleChange.newRole}`);
      
      // Refresh all users to get updated roles
      const fetchedUsers = await fetchUsers();
      setAllUsers(fetchedUsers);
      
      // Reset form
      setRoleChange({
        userId: '',
        newRole: ROLES[0]
      });
    } catch (err) {
      console.error('Error changing user role:', err);
      setError('Failed to change user role');
    } finally {
      setLoading(false);
    }
  };

  // Render access denied if not admin or superadmin
  if (userRole !== 'admin' && userRole !== 'superadmin') {
    return (
      <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded">
        <p className="font-bold">Access Denied</p>
        <p>You do not have permission to access the admin page.</p>
      </div>
    );
  }

  return (
    <div className="container mx-auto px-4 py-8">
      <h1 className="text-2xl font-bold mb-6">Admin Dashboard</h1>
      
      {error && (
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
          {error}
          <button className="float-right font-bold" onClick={() => setError(null)}>×</button>
        </div>
      )}
      
      {success && (
        <div className="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded mb-4">
          {success}
          <button className="float-right font-bold" onClick={() => setSuccess(null)}>×</button>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* User Role Management (Superadmin only) */}
        {userRole === 'superadmin' && (
          <div className="bg-white p-6 rounded-lg shadow">
            <h2 className="text-xl font-semibold mb-4">User Role Management</h2>
            <form onSubmit={handleRoleChange}>
              <div className="mb-4">
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Select User
                </label>
                <select
                  value={roleChange.userId}
                  onChange={(e) => setRoleChange({...roleChange, userId: e.target.value})}
                  className="block w-full rounded-md border border-gray-300 shadow-sm px-3 py-2"
                  required
                >
                  <option value="">Select a user</option>
                  {allUsers.map(user => (
                    <option key={user.id} value={user.id}>
                      {user.name} ({user.role || 'user'})
                    </option>
                  ))}
                </select>
              </div>
              
              <div className="mb-4">
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  New Role
                </label>
                <select
                  value={roleChange.newRole}
                  onChange={(e) => setRoleChange({...roleChange, newRole: e.target.value})}
                  className="block w-full rounded-md border border-gray-300 shadow-sm px-3 py-2"
                  required
                >
                  {ROLES.map(role => (
                    <option key={role} value={role}>
                      {role.charAt(0).toUpperCase() + role.slice(1)}
                    </option>
                  ))}
                </select>
              </div>
              
              <button
                type="submit"
                className="bg-indigo-600 text-white px-4 py-2 rounded-md hover:bg-indigo-700 disabled:opacity-50"
                disabled={loading}
              >
                {loading ? 'Updating...' : 'Update Role'}
              </button>
            </form>
          </div>
        )}

        {/* Permission Management */}
        <div className="bg-white p-6 rounded-lg shadow">
          <h2 className="text-xl font-semibold mb-4">Grant Permission</h2>
          <form onSubmit={handleGrantPermission}>
            <div className="mb-4">
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Grant To
              </label>
              <select
                value={newPermission.granteeId}
                onChange={(e) => setNewPermission({...newPermission, granteeId: e.target.value})}
                className="block w-full rounded-md border border-gray-300 shadow-sm px-3 py-2"
                required
              >
                <option value="">Select a user</option>
                {allUsers.map(user => (
                  <option key={user.id} value={user.id}>
                    {user.name}
                  </option>
                ))}
              </select>
            </div>
            
            <div className="mb-4">
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Resource Type
              </label>
              <select
                value={newPermission.resourceType}
                onChange={(e) => setNewPermission({...newPermission, resourceType: e.target.value})}
                className="block w-full rounded-md border border-gray-300 shadow-sm px-3 py-2"
                required
              >
                {RESOURCE_TYPES.map(type => (
                  <option key={type} value={type}>
                    {type.replace('_', ' ').charAt(0).toUpperCase() + type.replace('_', ' ').slice(1)}
                  </option>
                ))}
              </select>
            </div>
            
            <div className="mb-4">
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Permission Type
              </label>
              <select
                value={newPermission.permissionType}
                onChange={(e) => setNewPermission({...newPermission, permissionType: e.target.value})}
                className="block w-full rounded-md border border-gray-300 shadow-sm px-3 py-2"
                required
              >
                {PERMISSION_TYPES.map(type => (
                  <option key={type} value={type}>
                    {type.charAt(0).toUpperCase() + type.slice(1)}
                  </option>
                ))}
              </select>
            </div>
            
            <div className="mb-4">
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Expires At (Optional)
              </label>
              <input
                type="date"
                value={newPermission.expiresAt}
                onChange={(e) => setNewPermission({...newPermission, expiresAt: e.target.value})}
                className="block w-full rounded-md border border-gray-300 shadow-sm px-3 py-2"
              />
            </div>
            
            <button
              type="submit"
              className="bg-indigo-600 text-white px-4 py-2 rounded-md hover:bg-indigo-700 disabled:opacity-50"
              disabled={loading}
            >
              {loading ? 'Granting...' : 'Grant Permission'}
            </button>
          </form>
        </div>

        {/* View User Permissions */}
        <div className="bg-white p-6 rounded-lg shadow lg:col-span-2">
          <h2 className="text-xl font-semibold mb-4">View User Permissions</h2>
          
          <div className="mb-4">
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Select User
            </label>
            <select
              value={selectedUser}
              onChange={(e) => loadUserPermissions(e.target.value)}
              className="block w-full rounded-md border border-gray-300 shadow-sm px-3 py-2"
            >
              <option value="">Select a user</option>
              {allUsers.map(user => (
                <option key={user.id} value={user.id}>
                  {user.name}
                </option>
              ))}
            </select>
          </div>
          
          {selectedUser && (
            <>
              <h3 className="font-medium text-lg my-2">
                Permissions for {allUsers.find(u => u.id === selectedUser)?.name || selectedUser}
              </h3>
              
              {userPermissions.length === 0 ? (
                <p className="text-gray-500 italic">No permissions found</p>
              ) : (
                <div className="overflow-x-auto">
                  <table className="min-w-full divide-y divide-gray-200">
                    <thead className="bg-gray-50">
                      <tr>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Resource Type
                        </th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Permission Type
                        </th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Granted By
                        </th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Granted On
                        </th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Expires On
                        </th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Actions
                        </th>
                      </tr>
                    </thead>
                    <tbody className="bg-white divide-y divide-gray-200">
                      {userPermissions.map(permission => {
                        const owner = allUsers.find(u => u.id === permission.ownerUserID);
                        const createdDate = new Date(permission.createdAt).toLocaleDateString();
                        const expiresDate = permission.expiresAt 
                          ? new Date(permission.expiresAt).toLocaleDateString() 
                          : 'Never';
                          
                        return (
                          <tr key={permission.id}>
                            <td className="px-6 py-4 whitespace-nowrap">
                              {permission.resourceType.replace('_', ' ')}
                            </td>
                            <td className="px-6 py-4 whitespace-nowrap">
                              {permission.permissionType}
                            </td>
                            <td className="px-6 py-4 whitespace-nowrap">
                              {owner?.name || permission.ownerUserID}
                            </td>
                            <td className="px-6 py-4 whitespace-nowrap">
                              {createdDate}
                            </td>
                            <td className="px-6 py-4 whitespace-nowrap">
                              {expiresDate}
                            </td>
                            <td className="px-6 py-4 whitespace-nowrap">
                              <button
                                onClick={() => handleRevokePermission(permission)}
                                className="text-red-600 hover:text-red-900"
                                disabled={loading}
                              >
                                Revoke
                              </button>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  );
}

export default AdminPage; 