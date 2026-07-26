# Reusable Service module for any Hexchess service deployments

# Locals
locals {
  prefix = var.project
}

#  ECS Service
resource "aws_ecs_service" "app" {
  name            = "${local.prefix}-service-${var.app_name}-app"
  cluster         = var.cluster_arn
  task_definition = aws_ecs_task_definition.app.arn
  desired_count   = var.desired_task_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.private_subnets_ids # tasks run in private subnets
    security_groups  = [var.security_group_id]
    assign_public_ip = false
  }
}

# ECS Task Definition
resource "aws_ecs_task_definition" "app" {
  family                   = "${local.prefix}-task-definition-${var.app_name}-app"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]

  execution_role_arn = var.execution_role_arn
  task_role_arn      = var.task_role_arn

  cpu    = var.task_cpu
  memory = var.task_memory

  container_definitions = jsonencode([{
    name  = var.project
    image = "${var.container_image}:${var.image_tag}"

    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = aws_cloudwatch_log_group.app.name
        awslogs-region        = var.aws_region
        awslogs-stream-prefix = "ecs"
      }
    }

    environment = [
      for k, v in var.env_vars : {
        name = k, value = v
      }
    ]
  }])
}

# ALB Rules

# resource "aws_lb_target_group" "app" {
#   name        = "${var.project}-${var.environment}-tg"
#   port        = var.app_port
#   protocol    = "HTTP"
#   vpc_id      = var.vpc_id
#   target_type = "ip"  # required for Fargate
#
#   health_check {
#     path                = var.healthcheck_path
#     healthy_threshold   = 2
#     unhealthy_threshold = 3
#     interval            = 30
#   }
# }
#
# resource "aws_lb_listener_rule" "static" {
#   listener_arn = var.alb_listener_arn
#   priority     = var.base_priority + 1
#
#   action {
#     type             = "forward"
#     target_group_arn = aws_lb_target_group.app.arn
#   }
#
#   condition {
#     path_pattern {
#       values = var.path_patterns
#     }
#   }
# }

# CloudWatch Log Group
resource "aws_cloudwatch_log_group" "app" {
  name              = "/aws/ecs/${var.project}/${var.app_name}/${trimprefix(var.image_tag, "sha256:", )}"
  retention_in_days = 1 # We should be using Loki for long term log storage
}