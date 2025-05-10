/**
 * Utility functions for formatting values
 */

/**
 * Format a date string to a human-readable format
 * @param dateString - ISO date string or Date object
 * @returns Formatted date string (MM/DD/YYYY)
 */
export function formatDate(dateString: string | Date | undefined): string {
  if (!dateString) return '';
  
  try {
    const date = typeof dateString === 'string' ? new Date(dateString) : dateString;
    return date.toLocaleDateString();
  } catch (error) {
    console.error('Error formatting date:', error);
    return '';
  }
}

/**
 * Format a number as currency (USD)
 * @param amount - Number to format
 * @returns Formatted money string
 */
export function formatMoney(amount: number): string {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
  }).format(amount);
}

/**
 * Format a number as percentage
 * @param value - Number to format as percentage
 * @param decimals - Number of decimal places
 * @returns Formatted percentage string
 */
export function formatPercentage(value: number, decimals = 0): string {
  return new Intl.NumberFormat('en-US', {
    style: 'percent',
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  }).format(value / 100);
} 