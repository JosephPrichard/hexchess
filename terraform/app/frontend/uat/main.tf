variable "commit_sha" {
  description = "Commit SHA for this specific deployment"
  type        = string
}

variable "rollout" {
  description = "Rollout kind of app deployment (e.g blue, green, switch)"
  type        = string
}

variable "green_switch_weight" {
  description = "The % of traffic to be directed to the green app when rollout is set to 'switch'"
  type        = string
  default     = 25
}

locals {
  infra = data.terraform_remote_state.infra.outputs
}

module "main" {
  source = "../../global/service"

  vpc_id              = local.infra.vpc_id
  private_subnets_ids = local.infra.private_subnets_ids
  cluster_arn         = local.infra.cluster_arn
  alb_listener_arns   = [local.infra.public_alb_listener_arn, local.infra.private_alb_listener_arn]
  security_group_id   = local.infra.listener_security_group_id
  execution_role_arn  = local.infra.ecs_task_execution_role_arn
  task_role_arn       = local.infra.ecs_task_frontend_role_arn

  project     = "hexchess"
  environment = "uat"
  aws_region  = "us-east-1"
  app_name    = "frontend"

  desired_task_count = 1
  task_cpu           = 256
  task_memory        = 512

  app_port         = 5173
  healthcheck_path = "/healthcheck"
  base_priority    = 1000

  rollout             = var.rollout
  green_switch_weight = var.green_switch_weight
  green_condition     = "host_header"

  container_image = "938864279852.dkr.ecr.us-east-1.amazonaws.com/releases/hexchess/frontend"
  image_tag       = var.commit_sha

  env_vars = {
    PUBLIC_ROLLOUT: var.rollout,
    PUBLIC_ACTIVE_PROFILE: "test",
    PUBLIC_APP_BASE_URL: "https://uat.hexagonchess.app",
    INTERNAL_BACKEND_BASE_URL: "http://internal-hexchess-alb-private-965831337.us-east-1.elb.amazonaws.com"
  }
}