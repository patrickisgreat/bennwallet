provider "aws" {
  region = "us-west-2" # Update to your preferred region
}

terraform {
  required_version = ">= 1.0.0"
  
  backend "s3" {
    bucket         = "bennwallet-terraform-state" # Update with your state bucket
    key            = "prod/terraform.tfstate"
    region         = "us-west-2" # Update to your preferred region
    encrypt        = true
    dynamodb_table = "terraform-lock"
  }
  
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

locals {
  environment = "prod"
  app_name    = "bennwallet"
}

module "vpc" {
  source = "../../modules/vpc"
  
  vpc_name          = "${local.app_name}-${local.environment}-vpc"
  environment       = local.environment
  vpc_cidr          = "10.0.0.0/16"
  availability_zones = ["us-west-2a", "us-west-2b"] # Update to your preferred AZs
}

module "database" {
  source = "../../modules/database"
  
  identifier       = "${local.app_name}-${local.environment}"
  engine_version   = "14"
  instance_class   = "db.t3.micro"
  allocated_storage = 20
  database_name    = "bennwallet"
  username         = "postgres"
  password         = var.db_password
  vpc_id           = module.vpc.vpc_id
  subnet_ids       = module.vpc.private_subnet_ids
  environment      = local.environment
}

module "app" {
  source = "../../modules/app"
  
  app_name          = local.app_name
  environment       = local.environment
  vpc_id            = module.vpc.vpc_id
  subnet_ids        = module.vpc.public_subnet_ids
  container_image   = "${var.aws_account_id}.dkr.ecr.${var.aws_region}.amazonaws.com/${local.app_name}-${local.environment}:latest"
  container_port    = 8080
  desired_count     = 1
  cpu               = 256
  memory            = 512
  database_url      = module.database.connection_string
  ynab_api_token    = var.ynab_api_token
  ynab_budget_id    = var.ynab_budget_id
  ynab_account_id   = var.ynab_account_id
  firebase_credentials = var.firebase_credentials
}

output "ecr_repository_url" {
  description = "URL of the ECR repository"
  value       = module.app.ecr_repository_url
}

output "database_endpoint" {
  description = "The RDS instance endpoint"
  value       = module.database.rds_hostname
}

output "rds_port" {
  description = "The database port"
  value       = module.database.rds_port
} 