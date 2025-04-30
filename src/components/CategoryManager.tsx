import { useState, useEffect } from 'react';

interface Category {
  id: number;
  name: string;
}

function CategoryManager() {
  const [categories, setCategories] = useState<Category[]>([]);
  const [newCategory, setNewCategory] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load categories from the server
  useEffect(() => {
    fetchCategories();
  }, []);

  const fetchCategories = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch('/categories');
      if (!res.ok) {
        throw new Error(`Server responded with ${res.status}`);
      }
      const data = await res.json();
      
      // Ensure we're setting an array even if API returns null or undefined
      if (Array.isArray(data)) {
        setCategories(data);
      } else {
        console.warn('Categories API did not return an array:', data);
        setCategories([]);
      }
    } catch (err) {
      console.error('Error fetching categories:', err);
      setError('Failed to load categories');
      setCategories([]);
    } finally {
      setLoading(false);
    }
  };

  const addCategory = async () => {
    if (!newCategory.trim()) return;
    
    setLoading(true);
    try {
      const res = await fetch('/categories', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: newCategory.trim() }),
      });

      if (!res.ok) {
        throw new Error(`Server responded with ${res.status}`);
      }

      setNewCategory('');
      fetchCategories(); // Refresh categories after adding
    } catch (err) {
      console.error('Error adding category:', err);
      setError('Failed to add category');
    } finally {
      setLoading(false);
    }
  };

  const deleteCategory = async (id: number) => {
    if (!confirm('Delete this category?')) return;

    setLoading(true);
    try {
      const res = await fetch(`/categories/${id}`, {
        method: 'DELETE',
      });

      if (!res.ok) {
        throw new Error(`Server responded with ${res.status}`);
      }

      fetchCategories(); // Refresh categories after deleting
    } catch (err) {
      console.error('Error deleting category:', err);
      setError('Failed to delete category');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="bg-white p-4 rounded shadow mb-6">
      <h2 className="text-xl font-bold mb-2">Manage Categories</h2>
      
      {error && (
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
          {error}
        </div>
      )}

      <div className="flex gap-2 mb-4">
        <input
          type="text"
          placeholder="New category"
          value={newCategory}
          onChange={(e) => setNewCategory(e.target.value)}
          className="border rounded p-2 flex-1 bg-white text-black"
          disabled={loading}
        />
        <button
          onClick={addCategory}
          className="bg-green-500 text-white p-2 rounded"
          disabled={loading}
        >
          {loading ? 'Adding...' : 'Add'}
        </button>
      </div>

      {loading && <p className="text-gray-500">Loading...</p>}
      
      {!loading && categories.length === 0 && (
        <p className="text-gray-500">No categories found. Add one to get started.</p>
      )}

      <ul className="space-y-2">
        {categories.map((category) => (
          <li key={category.id} className="flex justify-between items-center">
            <span>{category.name}</span>
            <button
              onClick={() => deleteCategory(category.id)}
              className="text-red-500 hover:underline"
              disabled={loading}
            >
              Delete
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}

export default CategoryManager;
