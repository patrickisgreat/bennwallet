import { useState, useEffect } from 'react';
import { Category } from '../types/category';
import { useUser } from '../context/UserContext';
import { api } from '../utils/api';

function CategoriesPage() {
  const [categories, setCategories] = useState<Category[]>([]);
  const [newCategory, setNewCategory] = useState({ name: '', description: '' });
  const [editingCategory, setEditingCategory] = useState<Category | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const { currentUser } = useUser();

  useEffect(() => {
    if (currentUser) {
      loadCategories();
    }
  }, [currentUser]);

  const loadCategories = async () => {
    setLoading(true);
    try {
      const response = await api.get('/categories', {
        params: { userId: currentUser?.id }
      });
      
      // Ensure we always set an array, even if API returns null or undefined
      if (Array.isArray(response.data)) {
        setCategories(response.data);
      } else {
        console.warn('Categories API did not return an array:', response.data);
        setCategories([]);
      }
    } catch (error) {
      console.error('Error loading categories:', error);
      setError('Failed to load categories');
      setCategories([]);
    } finally {
      setLoading(false);
    }
  };

  const handleAddCategory = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!currentUser) return;

    setLoading(true);
    try {
      const response = await api.post('/categories', {
        ...newCategory,
        userId: currentUser.id
      });
      
      // Make sure we're adding to an array
      if (Array.isArray(categories)) {
        setCategories([...categories, response.data]);
      } else {
        setCategories([response.data]);
      }
      
      setNewCategory({ name: '', description: '' });
      setError(null);
    } catch (error) {
      console.error('Error adding category:', error);
      setError('Failed to add category');
    } finally {
      setLoading(false);
    }
  };

  const handleUpdateCategory = async (category: Category) => {
    if (!currentUser) return;

    setLoading(true);
    try {
      console.log('Updating category with data:', category);
      
      const response = await api.put(`/categories/${category.id}`, {
        ...category,
        userId: currentUser.id
      });
      
      console.log('Update category response:', response);
      
      // Make sure we're updating an array
      if (Array.isArray(categories)) {
        if (response.data) {
          setCategories(categories.map(c => c.id === category.id ? response.data : c));
          console.log('Category updated successfully');
        } else {
          console.warn('Update response did not contain data');
        }
      }
      
      setEditingCategory(null);
      setError(null);
    } catch (error: any) {
      console.error('Error updating category:', error);
      const errorMessage = error.response?.data?.message || 'Failed to update category';
      setError(`Error: ${errorMessage}`);
    } finally {
      setLoading(false);
    }
  };

  const handleDeleteCategory = async (id: number) => {
    if (!currentUser || !window.confirm('Are you sure you want to delete this category?')) return;

    setLoading(true);
    try {
      await api.delete(`/categories/${id}`, {
        params: { userId: currentUser.id }
      });
      
      // Make sure we're filtering an array
      if (Array.isArray(categories)) {
        setCategories(categories.filter(c => c.id !== id));
      }
      
      setError(null);
    } catch (error) {
      console.error('Error deleting category:', error);
      setError('Failed to delete category');
    } finally {
      setLoading(false);
    }
  };

  if (!currentUser) {
    return <div>Please log in to manage categories</div>;
  }

  return (
    <div className="max-w-4xl mx-auto p-4">
      <h1 className="text-2xl font-bold mb-4">Manage Categories</h1>

      {error && (
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
          {error}
          <button 
            className="float-right font-bold"
            onClick={() => setError(null)}
          >
            &times;
          </button>
        </div>
      )}

      <form onSubmit={handleAddCategory} className="mb-6">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700">Name</label>
            <input
              type="text"
              value={newCategory.name}
              onChange={(e) => setNewCategory({ ...newCategory, name: e.target.value })}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
              required
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Description</label>
            <input
              type="text"
              value={newCategory.description}
              onChange={(e) => setNewCategory({ ...newCategory, description: e.target.value })}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
            />
          </div>
        </div>
        <button
          type="submit"
          className="mt-4 bg-indigo-600 text-white px-4 py-2 rounded-md hover:bg-indigo-700"
        >
          Add Category
        </button>
      </form>

      {loading ? (
        <div className="text-center p-4">Loading categories...</div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {Array.isArray(categories) && categories.length > 0 ? (
            categories.map((category) => (
              <div
                key={category.id}
                className="bg-white p-4 rounded-lg shadow"
                style={{ borderLeft: `4px solid ${category.color || '#4F46E5'}` }}
              >
                {editingCategory?.id === category.id ? (
                  <form
                    onSubmit={(e) => {
                      e.preventDefault();
                      handleUpdateCategory(editingCategory);
                    }}
                  >
                    <input
                      type="text"
                      value={editingCategory.name}
                      onChange={(e) =>
                        setEditingCategory({ ...editingCategory, name: e.target.value })
                      }
                      className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 mb-2"
                      required
                    />
                    <input
                      type="text"
                      value={editingCategory.description}
                      onChange={(e) =>
                        setEditingCategory({ ...editingCategory, description: e.target.value })
                      }
                      className="block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 mb-2"
                    />
                    <div className="flex gap-2">
                      <button
                        type="submit"
                        className="bg-indigo-600 text-white px-3 py-1 rounded-md hover:bg-indigo-700"
                      >
                        Save
                      </button>
                      <button
                        type="button"
                        onClick={() => setEditingCategory(null)}
                        className="bg-gray-200 text-gray-700 px-3 py-1 rounded-md hover:bg-gray-300"
                      >
                        Cancel
                      </button>
                    </div>
                  </form>
                ) : (
                  <>
                    <h3 className="font-medium">{category.name}</h3>
                    {category.description && (
                      <p className="text-gray-600 text-sm mt-1">{category.description}</p>
                    )}
                    <div className="mt-2 flex gap-2">
                      <button
                        onClick={() => setEditingCategory(category)}
                        className="text-indigo-600 hover:text-indigo-800"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => handleDeleteCategory(category.id)}
                        className="text-red-600 hover:text-red-800"
                      >
                        Delete
                      </button>
                    </div>
                  </>
                )}
              </div>
            ))
          ) : (
            <div className="col-span-3 text-center p-4 text-gray-500">
              No categories found. Add one to get started.
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export default CategoriesPage; 