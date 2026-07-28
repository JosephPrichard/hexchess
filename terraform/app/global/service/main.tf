# Reusable Service module for any Hexchess service deployments

locals {
  prefix = var.project

  container_image = "${var.container_image}:${var.image_tag}"

  enable_alb         = var.healthcheck_path != ""
  enable_green_app   = var.rollout == "green" || var.rollout == "switch"
  is_blue_deployment = !local.enable_green_app

  environment = [
    for k, v in var.env_vars : {
      name = k, value = v
    }
  ]
}

### ECS Service (Blue)
locals {
  container_name_blue = "${local.prefix}-${var.app_name}-container"
}

resource "aws_ecs_service" "blue" {
  name                   = "${local.prefix}-${var.app_name}-service"
  cluster                = var.cluster_arn
  task_definition        = aws_ecs_task_definition.blue.arn
  desired_count          = var.desired_task_count
  launch_type            = "FARGATE"

  health_check_grace_period_seconds = 30
  enable_execute_command            = true

  dynamic "load_balancer" {
    for_each = local.enable_alb ? [1] : []
    content {
      target_group_arn = aws_lb_target_group.blue.arn
      container_name   = local.container_name_blue
      container_port   = var.app_port
    }
  }

  network_configuration {
    subnets          = var.private_subnets_ids # tasks run in private subnets
    security_groups  = [var.security_group_id]
    assign_public_ip = false
  }
}

# ECS Task Definition
locals {
  blue_task_definition_name = "${local.prefix}-${var.app_name}-task-definition"

  # Find the previous task definition container image for the existing blue app, default to a new deployment otherwise
  # previous_blue_container_image      = try(
  #   jsondecode(data.aws_ecs_task_definition.previous_blue.container_definitions).image,
  #   local.container_image
  # )
}

# data "aws_ecs_task_definition" "previous_blue" {
#   task_definition = local.blue_task_definition_name
# }

resource "aws_ecs_task_definition" "blue" {
  family                   = local.blue_task_definition_name
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]

  execution_role_arn = var.execution_role_arn
  task_role_arn      = var.task_role_arn

  cpu    = var.task_cpu
  memory = var.task_memory

  container_definitions = jsonencode([{
    name = local.container_name_blue

    # We can take the new container_image on a blue deployment, but we should take the old container definition image otherwise
    # image = local.is_blue_deployment ? local.container_image : local.previous_blue_container_image
    image = local.container_image

    essential = true
    portMappings = [
      {
        containerPort = var.app_port
        hostPort      = var.app_port
      }
    ]

    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = aws_cloudwatch_log_group.blue.name
        awslogs-region        = var.aws_region
        awslogs-stream-prefix = "ecs"
      }
    }

    environment = local.environment
  }])
}

### ECS Service (Green)
locals {
  container_name_green = "${local.prefix}-${var.app_name}-container-green"
}

resource "aws_ecs_service" "green" {
  count = local.enable_green_app ? 1 : 0

  name                   = "${local.prefix}-${var.app_name}-service-green"
  cluster                = var.cluster_arn
  task_definition        = aws_ecs_task_definition.green.arn
  desired_count          = var.desired_task_count
  launch_type            = "FARGATE"

  health_check_grace_period_seconds = 30
  enable_execute_command            = true

  dynamic "load_balancer" {
    for_each = local.enable_alb ? [1] : []
    content {
      target_group_arn = aws_lb_target_group.green.arn
      container_name   = local.container_name_green
      container_port   = var.app_port
    }
  }

  network_configuration {
    subnets          = var.private_subnets_ids # tasks run in private subnets
    security_groups  = [var.security_group_id]
    assign_public_ip = false
  }
}

# ECS Task Definition
resource "aws_ecs_task_definition" "green" {
  family                   = "${local.prefix}-${var.app_name}-task-definition-green"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]

  execution_role_arn = var.execution_role_arn
  task_role_arn      = var.task_role_arn

  cpu    = var.task_cpu
  memory = var.task_memory

  container_definitions = jsonencode([{
    name  = local.container_name_green
    # Always take the new container_image for a green deployment, green app has the latest code we are testing in this deployment
    image = local.container_image

    portMappings = [
      {
        containerPort = var.app_port
        hostPort      = var.app_port
      }
    ]

    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = aws_cloudwatch_log_group.green.name
        awslogs-region        = var.aws_region
        awslogs-stream-prefix = "ecs"
      }
    }

    environment = local.environment
  }])
}

### ALB Blue Rules

locals {
  blue_offset = 50
}

resource "aws_lb_target_group" "blue" {
  name        = "${local.prefix}-${var.app_name}-tg"
  protocol    = "HTTP"
  port        = var.app_port
  vpc_id      = var.vpc_id
  target_type = "ip"  # required for Fargate

  health_check {
    path                = var.healthcheck_path
    healthy_threshold   = 2
    unhealthy_threshold = 3
    interval            = 30
  }
}

resource "aws_lb_listener_rule" "blue" {
  count = local.enable_alb ? 1 : 0

  listener_arn = var.alb_listener_arn
  priority     = var.base_priority + local.blue_offset

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.blue.arn
  }

  condition {
    path_pattern {
      values = var.path_patterns
    }
  }
}

### ALB Green Rules

locals {
  green_offset       = 25
}

resource "aws_lb_target_group" "green" {
  name        = "${local.prefix}-${var.app_name}-green-tg"
  protocol    = "HTTP"
  port        = var.app_port
  vpc_id      = var.vpc_id
  target_type = "ip"  # required for Fargate

  health_check {
    path                = var.healthcheck_path
    healthy_threshold   = 2
    unhealthy_threshold = 3
    interval            = 30
  }
}

# The main difference between a green rollout and a switch rollout is how we decide what makes it to the green app
# In a green rollout, the traffic that makes it to the green app is marked by the caller through a header explicitly
# In a switch rollout, the caller does not decide, instead a certain % of traffic makes it to the green and rest makes it to blue
# So in a green rollout, the developer controls when to hit green (and therefore customers will not make it to blue)
# In a switch rollout, the infrastructure controls when to hit green, so only a limited amount of customers are exposed to new code
resource "aws_lb_listener_rule" "green" {
  count = local.enable_alb && local.enable_green_app ? 1 : 0

  listener_arn = var.alb_listener_arn
  priority     = var.base_priority + local.green_offset

  action {
    type             = "forward"
    forward {
      target_group {
        arn    = aws_lb_target_group.green.arn
        # In a green rollout, always forward to the green app, in a switch, a % of the traffic is redirected to green app
        weight = var.rollout == "green" ? 100 : var.green_switch_weight
      }
      target_group {
        arn    = aws_lb_target_group.blue.arn
        # In a green rollout, we never route to the blue app in this rule (let the blue rule handle that), in a switch a % of traffic is redirected to blue app
        weight = var.rollout == "green" ? 0 : (100 - var.green_switch_weight)
      }
    }
  }

  condition {
    path_pattern {
      values = var.path_patterns
    }
  }
  condition {
    # In a green rollout, this rule always intercepts any traffic marked as green
    # In a switch rollout, this rule any traffic regardless of how it is marked, but then uses the weights to decide what makes it to blue/green
    dynamic "http_header" {
      for_each = var.rollout == "green" ? [1] : []
      content {
        http_header_name = "Rollout"
        values           = ["Green*", "GREEN*", "green*"]
      }
    }
  }
}

### CloudWatch Log Group
# Keep retention_in_days low, we should be using grafana for logs
locals {
  aws_cloudwatch_log_group_name = "/aws/ecs/${var.project}/${var.app_name}"
}

resource "aws_cloudwatch_log_group" "blue" {
  name              = "${local.aws_cloudwatch_log_group_name}/blue"
  retention_in_days = 1
}

resource "aws_cloudwatch_log_group" "green" {
  name              = "${local.aws_cloudwatch_log_group_name}/green"
  retention_in_days = 1
}