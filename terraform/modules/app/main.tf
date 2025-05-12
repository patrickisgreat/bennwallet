variable "app_name" {
  description = "Name of the application"
  type        = string
}

variable "environment" {
  description = "Environment (dev, prod, etc)"
  type        = string
}

variable "vpc_id" {
  description = "The VPC ID"
  type        = string
}

variable "subnet_ids" {
  description = "List of subnet IDs for the ECS tasks"
  type        = list(string)
}

variable "container_image" {
  description = "Docker image for the application"
  type        = string
}

variable "container_port" {
  description = "Port exposed by the container"
  type        = number
  default     = 8080
}

variable "desired_count" {
  description = "Number of instances of the task to run"
  type        = number
  default     = 1
}

variable "cpu" {
  description = "The number of cpu units to reserve for the container"
  type        = number
  default     = 256
}

variable "memory" {
  description = "The amount (in MiB) of memory to reserve for the container"
  type        = number
  default     = 512
}

variable "database_url" {
  description = "Database connection string"
  type        = string
  sensitive   = true
}

variable "ynab_api_token" {
  description = "YNAB API Token"
  type        = string
  sensitive   = true
  default     = ""
}

variable "ynab_budget_id" {
  description = "YNAB Budget ID"
  type        = string
  sensitive   = true
  default     = ""
}

variable "ynab_account_id" {
  description = "YNAB Account ID"
  type        = string
  sensitive   = true
  default     = ""
}

variable "firebase_credentials" {
  description = "Firebase service account JSON"
  type        = string
  sensitive   = true
  default     = ""
}

resource "aws_ecr_repository" "app" {
  name = "${var.app_name}-${var.environment}"
  
  image_scanning_configuration {
    scan_on_push = true
  }
  
  tags = {
    Name        = "${var.app_name}-${var.environment}"
    Environment = var.environment
  }
}

resource "aws_cloudwatch_log_group" "app" {
  name = "/ecs/${var.app_name}-${var.environment}"
  
  tags = {
    Name        = "${var.app_name}-${var.environment}"
    Environment = var.environment
  }
}

resource "aws_security_group" "ecs" {
  name        = "${var.app_name}-${var.environment}-ecs-sg"
  description = "Allow traffic for ECS"
  vpc_id      = var.vpc_id

  ingress {
    from_port   = var.container_port
    to_port     = var.container_port
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name        = "${var.app_name}-${var.environment}-ecs-sg"
    Environment = var.environment
  }
}

resource "aws_iam_role" "ecs_task_execution" {
  name = "${var.app_name}-${var.environment}-ecs-execution-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
        }
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "ecs_task_execution" {
  role       = aws_iam_role.ecs_task_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_ecs_cluster" "main" {
  name = "${var.app_name}-${var.environment}"
  
  setting {
    name  = "containerInsights"
    value = "enabled"
  }
  
  tags = {
    Name        = "${var.app_name}-${var.environment}"
    Environment = var.environment
  }
}

resource "aws_ecs_task_definition" "app" {
  family                   = "${var.app_name}-${var.environment}"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.cpu
  memory                   = var.memory
  execution_role_arn       = aws_iam_role.ecs_task_execution.arn

  container_definitions = jsonencode([
    {
      name      = "${var.app_name}-${var.environment}"
      image     = var.container_image
      essential = true
      
      portMappings = [
        {
          containerPort = var.container_port
          hostPort      = var.container_port
          protocol      = "tcp"
        }
      ]
      
      environment = [
        {
          name  = "NODE_ENV"
          value = var.environment
        },
        {
          name  = "ENVIRONMENT"
          value = var.environment
        },
        {
          name  = "PORT"
          value = tostring(var.container_port)
        }
      ]
      
      secrets = [
        {
          name      = "DATABASE_URL"
          valueFrom = aws_ssm_parameter.db_url.arn
        },
        {
          name      = "YNAB_API_TOKEN"
          valueFrom = aws_ssm_parameter.ynab_api_token.arn
        },
        {
          name      = "YNAB_BUDGET_ID"
          valueFrom = aws_ssm_parameter.ynab_budget_id.arn
        },
        {
          name      = "YNAB_ACCOUNT_ID"
          valueFrom = aws_ssm_parameter.ynab_account_id.arn
        },
        {
          name      = "FIREBASE_SERVICE_ACCOUNT_JSON"
          valueFrom = aws_ssm_parameter.firebase_credentials.arn
        }
      ]
      
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.app.name
          "awslogs-region"        = "us-west-2" # Update to your region
          "awslogs-stream-prefix" = "ecs"
        }
      }
    }
  ])

  tags = {
    Name        = "${var.app_name}-${var.environment}"
    Environment = var.environment
  }
}

resource "aws_ssm_parameter" "db_url" {
  name        = "/${var.app_name}/${var.environment}/DATABASE_URL"
  description = "Database URL for ${var.app_name} ${var.environment} environment"
  type        = "SecureString"
  value       = var.database_url

  tags = {
    Environment = var.environment
  }
}

resource "aws_ssm_parameter" "ynab_api_token" {
  name        = "/${var.app_name}/${var.environment}/YNAB_API_TOKEN"
  description = "YNAB API Token"
  type        = "SecureString"
  value       = var.ynab_api_token

  tags = {
    Environment = var.environment
  }
}

resource "aws_ssm_parameter" "ynab_budget_id" {
  name        = "/${var.app_name}/${var.environment}/YNAB_BUDGET_ID"
  description = "YNAB Budget ID"
  type        = "SecureString"
  value       = var.ynab_budget_id

  tags = {
    Environment = var.environment
  }
}

resource "aws_ssm_parameter" "ynab_account_id" {
  name        = "/${var.app_name}/${var.environment}/YNAB_ACCOUNT_ID"
  description = "YNAB Account ID"
  type        = "SecureString"
  value       = var.ynab_account_id

  tags = {
    Environment = var.environment
  }
}

resource "aws_ssm_parameter" "firebase_credentials" {
  name        = "/${var.app_name}/${var.environment}/FIREBASE_SERVICE_ACCOUNT_JSON"
  description = "Firebase Service Account JSON"
  type        = "SecureString"
  value       = var.firebase_credentials

  tags = {
    Environment = var.environment
  }
}

resource "aws_ecs_service" "app" {
  name                               = "${var.app_name}-${var.environment}"
  cluster                            = aws_ecs_cluster.main.id
  task_definition                    = aws_ecs_task_definition.app.arn
  desired_count                      = var.desired_count
  launch_type                        = "FARGATE"
  scheduling_strategy                = "REPLICA"
  health_check_grace_period_seconds  = 60

  network_configuration {
    security_groups  = [aws_security_group.ecs.id]
    subnets          = var.subnet_ids
    assign_public_ip = true
  }

  tags = {
    Name        = "${var.app_name}-${var.environment}"
    Environment = var.environment
  }
}

output "ecr_repository_url" {
  description = "URL of the ECR repository"
  value       = aws_ecr_repository.app.repository_url
}

output "ecs_cluster_name" {
  description = "Name of the ECS cluster"
  value       = aws_ecs_cluster.main.name
}

output "ecs_service_name" {
  description = "Name of the ECS service"
  value       = aws_ecs_service.app.name
}

output "security_group_id" {
  description = "ID of the security group for the ECS tasks"
  value       = aws_security_group.ecs.id
} 