import { useState, useEffect } from 'react';

interface Category {
  id: number;
  name: string;
}

function CategoryManager() {
  const [categories, setCategories] = useState<Category[]>([]);
  const [newCategory, setNewCategory] = useState('');

  // Load categories from the server
  useEffect(() => {
    fetchCategories();
  }, []);

  const fetchCategories = async () => {
    const res = await fetch('http://localhost:4000/categories');
    const data = await res.json();
    setCategories(data);
  };

  const addCategory = async () => {
    if (!newCategory.trim()) return;

    const res = await fetch('http://localhost:4000/categories', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: newCategory.trim() }),
    });

    if (res.ok) {
      setNewCategory('');
      fetchCategories(); // Refresh categories after adding
    }
  };

  const deleteCategory = async (id: number) => {
    if (!confirm('Delete this category?')) return;

    const res = await fetch(`http://localhost:4000/categories/${id}`, {
      method: 'DELETE',
    });

    if (res.ok) {
      fetchCategories(); // Refresh categories after deleting
    }
  };

  return (
    <div className="bg-white p-4 rounded shadow mb-6">
      <h2 className="text-xl font-bold mb-2">Manage Categories</h2>

      <div className="flex gap-2 mb-4">
        <input
          type="text"
          placeholder="New category"
          value={newCategory}
          onChange={(e) => setNewCategory(e.target.value)}
          className="border rounded p-2 flex-1 bg-white text-black"
        />
        <button
          onClick={addCategory}
          className="bg-green-500 text-white p-2 rounded"
        >
          Add
        </button>
      </div>

      <ul className="space-y-2">
        {categories.map((category) => (
          <li key={category.id} className="flex justify-between items-center">
            <span>{category.name}</span>
            <button
              onClick={() => deleteCategory(category.id)}
              className="text-red-500 hover:underline"
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
