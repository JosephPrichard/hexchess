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

  project     = "hexchess"
  environment = "uat"
  aws_region  = "us-east-1"
  app_name    = "api"

  desired_task_count = 1
  task_cpu           = 256
  task_memory        = 512

  app_port         = 8080
  healthcheck_path = "/healthcheck"
  base_priority    = 500
  path_patterns    = ["*/api*"]

  rollout             = var.rollout
  green_switch_weight = var.green_switch_weight

  vpc_id              = local.infra.vpc_id
  private_subnets_ids = local.infra.private_subnets_ids
  cluster_arn         = local.infra.cluster_arn
  alb_listener_arn    = local.infra.alb_listener_arn
  security_group_id   = local.infra.listener_security_group_id
  execution_role_arn  = local.infra.ecs_task_execution_role_arn
  task_role_arn       = local.infra.ecs_task_app_role_arn

  container_image = "938864279852.dkr.ecr.us-east-1.amazonaws.com/releases/hexchess/api"
  image_tag       = var.commit_sha

  env_vars = {
    SERVER_PORT: "8080",

    DB_URL: "postgres://db_readwrite@hexchess-sor.cluster-c10fqqu5dr0g.us-east-1.rds.amazonaws.com/hexchess?sslmode=require",
    DB_READ_URL: "postgres://db_readwrite@hexchess-sor.cluster-ro-c10fqqu5dr0g.us-east-1.rds.amazonaws.com/hexchess?sslmode=require",

    REDIS_PUBSUB_NODE: "hexchess-pubsub.hl6d8h.0001.use1.cache.amazonaws.com:6379",
    REDIS_SOR_NODES:   join(",", [
      "hexchess-primary-cluster-0001-001.hexchess-primary-cluster.hl6d8h.memorydb.us-east-1.amazonaws.com:6379",
      "hexchess-primary-cluster-0001-002.hexchess-primary-cluster.hl6d8h.memorydb.us-east-1.amazonaws.com:6379",
      "hexchess-primary-cluster-0002-001.hexchess-primary-cluster.hl6d8h.memorydb.us-east-1.amazonaws.com:6379",
      "hexchess-primary-cluster-0002-002.hexchess-primary-cluster.hl6d8h.memorydb.us-east-1.amazonaws.com:6379"
    ]),

    PROFILE_BUCKET_NAME: "hexchess-profiles",

    ALLOWED_ORIGINS: "https://hexagonchess.app,https://green.hexagonchess.app,http://hexchess-nlb-public-4b82b58d6f4dbb05.elb.us-east-1.amazonaws.com",

    ACTIVE_PROFILE: "test"

    # OTEL_EXPORTER_OTLP_ENDPOINT: "localhost:3100"
  }
}