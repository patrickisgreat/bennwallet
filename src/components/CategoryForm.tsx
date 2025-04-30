import { useState, useEffect } from 'react';
import { Category } from '../types/category';

interface CategoryFormProps {
  onSubmit: (data: Omit<Category, 'id'>) => void;
  initialData?: Category | null;
  onCancel: () => void;
}

export default function CategoryForm({ onSubmit, initialData, onCancel }: CategoryFormProps) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');

  useEffect(() => {
    if (initialData) {
      setName(initialData.name);
      setDescription(initialData.description);
    } else {
      setName('');
      setDescription('');
    }
  }, [initialData]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit({ name, description });
    if (!initialData) {
      setName('');
      setDescription('');
    }
  };

  return (
    <form onSubmit={handleSubmit} className="bg-white p-4 rounded shadow mb-6">
      <div className="flex flex-col gap-4">
        <input
          type="text"
          placeholder="Category Name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="border rounded p-2"
          required
        />
        <input
          type="text"
          placeholder="Description"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          className="border rounded p-2"
          required
        />
        <div className="flex gap-2">
          <button type="submit" className="bg-blue-500 text-white p-2 rounded flex-1">
            {initialData ? 'Update Category' : 'Add Category'}
          </button>
          {initialData && (
            <button
              type="button"
              onClick={onCancel}
              className="bg-gray-400 text-white p-2 rounded"
            >
              Cancel
            </button>
          )}
        </div>
      </div>
    </form>
  );
} 