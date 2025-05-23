import { useState, useEffect, useRef } from 'react';
import { api, fetchCategories } from '../utils/api';
import { useUser } from '../context/UserContext';

interface YNABCategory {
  id: string;
  name: string;
  category_group_id: string;
  category_group_name: string;
}

interface CategoryGroup {
  id: string;
  name: string;
  categories: YNABCategory[];
}

interface HierarchicalCategorySelectorProps {
  value: string;
  onChange: (category: string) => void;
  className?: string;
}

export default function HierarchicalCategorySelector({ 
  value, 
  onChange,
  className = ""
}: HierarchicalCategorySelectorProps) {
  const { currentUser } = useUser();
  const [isOpen, setIsOpen] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  const [categoryGroups, setCategoryGroups] = useState<CategoryGroup[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);

  // Find the selected category display name
  const getSelectedCategoryDisplay = () => {
    if (!value) return 'Select a category';
    
    for (const group of categoryGroups || []) {
      if (!group || !group.categories) continue;
      const category = group.categories.find(c => c.name === value);
      if (category) {
        return `${group.name}: ${category.name}`;
      }
    }
    
    return value; // Fallback to just showing the value
  };

  // Load categories from the server
  useEffect(() => {
    if (currentUser) {
      loadCategories();
    }
  }, [currentUser]);

  // Close dropdown when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    }
    
    document.addEventListener('mousedown', handleClickOutside);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, []);

  const loadCategories = async () => {
    if (!currentUser) return;
    
    setLoading(true);
    setError(null);
    
    try {
      // First try to get hierarchical categories from the regular database
      try {
        const response = await api.get('/categories', {
          params: { userId: currentUser.id, hierarchical: 'true' }
        });
        
        if (response.data && Array.isArray(response.data) && response.data.length > 0) {
          setCategoryGroups(response.data);
          setLoading(false);
          return;
        }
      } catch (error) {
        console.error('Error loading hierarchical categories:', error);
      }
      
      // Fallback: Try to fetch YNAB categories
      try {
        const response = await api.get('/ynab/categories', {
          params: { userId: currentUser.id }
        });
        
        if (response.data && Array.isArray(response.data) && response.data.length > 0) {
          setCategoryGroups(response.data);
          setLoading(false);
          return;
        } else {
          console.warn('YNAB categories API did not return expected data:', response.data);
        }
      } catch (ynabError) {
        console.error('Error loading YNAB categories:', ynabError);
      }

      // Final fallback: Try to fetch flat database categories and group them by color
      try {
        const databaseCategories = await fetchCategories();
        
        if (databaseCategories && databaseCategories.length > 0) {
          // Group categories by color for visual organization
          const colorGroups: { [key: string]: YNABCategory[] } = {};
          
          databaseCategories.forEach(cat => {
            // Use the color as a group key, or "Other" if no color
            const colorKey = cat.color || "#CCCCCC";
            const colorName = getColorName(colorKey);
            
            if (!colorGroups[colorKey]) {
              colorGroups[colorKey] = [];
            }
            
            colorGroups[colorKey].push({
              id: String(cat.id),
              name: cat.name,
              category_group_id: colorKey,
              category_group_name: colorName
            });
          });
          
          // Convert the color groups to category groups
          const groups: CategoryGroup[] = Object.entries(colorGroups).map(([colorKey, categories]) => ({
            id: colorKey,
            name: getColorName(colorKey),
            categories
          }));
          
          setCategoryGroups(groups);
        } else {
          setError('No categories found. Please add categories in settings.');
          setCategoryGroups([]);
        }
      } catch (error) {
        console.error('Error loading flat categories:', error);
        setError('Failed to load categories');
        setCategoryGroups([]);
      }
    } catch (error) {
      console.error('Error loading categories:', error);
      setError('Failed to load categories');
      setCategoryGroups([]);
    } finally {
      setLoading(false);
    }
  };

  // Helper to convert color codes to readable names
  const getColorName = (colorCode: string): string => {
    const colorMap: {[key: string]: string} = {
      "#4CAF50": "Green",
      "#2196F3": "Blue",
      "#FFC107": "Yellow",
      "#9C27B0": "Purple",
      "#F44336": "Red",
      "#3F51B5": "Indigo",
      "#009688": "Teal",
      "#FF5722": "Orange",
      "#795548": "Brown",
      "#E91E63": "Pink",
      "#607D8B": "Gray",
      "#8BC34A": "Light Green",
      "#CCCCCC": "Other"
    };
    
    return colorMap[colorCode] || "Categories";
  };

  // Filter categories based on search term
  const filteredGroups = (categoryGroups || []).map(group => {
    if (!group || !group.categories) {
      return {
        ...group,
        categories: []
      };
    }
    
    const filteredCategories = group.categories.filter(category => 
      !searchTerm || 
      category.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      group.name.toLowerCase().includes(searchTerm.toLowerCase())
    );
    
    return {
      ...group,
      categories: filteredCategories
    };
  }).filter(group => group && group.categories && group.categories.length > 0);

  return (
    <div className={`relative ${className}`} ref={dropdownRef}>
      <div 
        className="border border-gray-300 rounded-md px-3 py-2 flex justify-between items-center cursor-pointer"
        onClick={() => setIsOpen(!isOpen)}
      >
        <div className="truncate">{getSelectedCategoryDisplay()}</div>
        <div>
          <svg className="h-5 w-5 text-gray-400" viewBox="0 0 20 20" fill="currentColor">
            <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
          </svg>
        </div>
      </div>
      
      {isOpen && (
        <div className="absolute mt-1 w-full bg-white border border-gray-300 rounded-md shadow-lg z-10">
          <div className="p-2 border-b">
            <input
              type="text"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              placeholder="Search categories..."
              className="w-full px-2 py-1 border border-gray-300 rounded-md"
              onClick={(e) => e.stopPropagation()}
            />
          </div>
          
          <div className="max-h-60 overflow-y-auto">
            {loading && (
              <div className="p-2 text-center text-gray-500">Loading categories...</div>
            )}
            
            {error && (
              <div className="p-2 text-center text-red-500">{error}</div>
            )}
            
            {!loading && !error && filteredGroups.length === 0 && (
              <div className="p-2 text-center text-gray-500">No categories found</div>
            )}
            
            {filteredGroups.map(group => (
              <div key={group.id} className="category-group">
                <div className="px-3 py-1 bg-gray-100 font-medium">{group.name}</div>
                <div>
                  {(group.categories || []).map(category => (
                    <div 
                      key={category.id}
                      className={`px-3 py-2 cursor-pointer hover:bg-gray-100 ${value === category.name ? 'bg-blue-50 text-blue-700' : ''}`}
                      onClick={() => {
                        onChange(category.name);
                        setIsOpen(false);
                        setSearchTerm('');
                      }}
                    >
                      {category.name}
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
} 