variable "identifier" {
  description = "The name of the RDS instance"
  type        = string
}

variable "engine_version" {
  description = "The Postgres engine version"
  type        = string
  default     = "14"
}

variable "instance_class" {
  description = "The instance type of the RDS instance"
  type        = string
  default     = "db.t3.micro"
}

variable "allocated_storage" {
  description = "The allocated storage in gigabytes"
  type        = number
  default     = 20
}

variable "database_name" {
  description = "The database name"
  type        = string
  default     = "bennwallet"
}

variable "username" {
  description = "Username for the master DB user"
  type        = string
  default     = "postgres"
}

variable "password" {
  description = "Password for the master DB user"
  type        = string
  sensitive   = true
}

variable "vpc_id" {
  description = "The VPC ID where the DB should be created"
  type        = string
}

variable "subnet_ids" {
  description = "A list of VPC subnet IDs"
  type        = list(string)
}

variable "environment" {
  description = "Environment (dev, prod, etc)"
  type        = string
}

resource "aws_security_group" "rds" {
  name        = "${var.identifier}-rds-sg"
  description = "Allow database traffic"
  vpc_id      = var.vpc_id

  ingress {
    description     = "PostgreSQL from App"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    cidr_blocks     = ["10.0.0.0/16"] # This should be your VPC CIDR
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name        = "${var.identifier}-rds-sg"
    Environment = var.environment
  }
}

resource "aws_db_subnet_group" "default" {
  name       = "${var.identifier}-subnet-group"
  subnet_ids = var.subnet_ids

  tags = {
    Name        = "${var.identifier}-subnet-group"
    Environment = var.environment
  }
}

resource "aws_db_instance" "postgres" {
  identifier             = var.identifier
  engine                 = "postgres"
  engine_version         = var.engine_version
  instance_class         = var.instance_class
  allocated_storage      = var.allocated_storage
  db_name                = var.database_name
  username               = var.username
  password               = var.password
  db_subnet_group_name   = aws_db_subnet_group.default.name
  vpc_security_group_ids = [aws_security_group.rds.id]
  publicly_accessible    = false
  skip_final_snapshot    = true
  
  tags = {
    Name        = var.identifier
    Environment = var.environment
  }
}

output "rds_hostname" {
  description = "RDS instance hostname"
  value       = aws_db_instance.postgres.address
}

output "rds_port" {
  description = "RDS instance port"
  value       = aws_db_instance.postgres.port
}

output "rds_username" {
  description = "RDS instance username"
  value       = aws_db_instance.postgres.username
}

output "database_name" {
  description = "Name of the database"
  value       = aws_db_instance.postgres.db_name
}

output "connection_string" {
  description = "PostgreSQL connection string"
  value       = "postgres://${aws_db_instance.postgres.username}:${var.password}@${aws_db_instance.postgres.address}:${aws_db_instance.postgres.port}/${aws_db_instance.postgres.db_name}"
  sensitive   = true
} 