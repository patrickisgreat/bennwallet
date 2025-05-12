variable "aws_region" {
  description = "AWS region to deploy to"
  type        = string
  default     = "us-west-2" # Update to your preferred region
}

variable "aws_account_id" {
  description = "AWS Account ID"
  type        = string
  # No default - this should be provided when applying
}

variable "db_password" {
  description = "Password for the database"
  type        = string
  sensitive   = true
  # No default - this should be provided when applying
}

variable "ynab_api_token" {
  description = "YNAB API Token"
  type        = string
  sensitive   = true
  # No default - this should be provided when applying
}

variable "ynab_budget_id" {
  description = "YNAB Budget ID"
  type        = string
  sensitive   = true
  # No default - this should be provided when applying
}

variable "ynab_account_id" {
  description = "YNAB Account ID"
  type        = string
  sensitive   = true
  # No default - this should be provided when applying
}

variable "firebase_credentials" {
  description = "Firebase service account JSON"
  type        = string
  sensitive   = true
  # No default - this should be provided when applying
} 