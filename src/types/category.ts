export interface Category {
  id: string;
  name: string;
  description?: string;
  color?: string;
  userId: string;
  categoryGroupId?: string;
  hidden?: boolean;
}

export interface CategoryGroup {
  id: string;
  name: string;
  categories: Category[];
} 