variable "commit_sha" {
  description = "Commit SHA for this specific deployment"
  type        = string
}

variable "rollout" {
  description = "Rollout kind of app deployment (e.g blue, green)"
  type        = string
}

locals {
  infra = data.terraform_remote_state.infra.outputs
}

module "main" {
  source = "../../global/service"

  private_subnets_ids = local.infra.private_subnets_ids
  cluster_arn         = local.infra.cluster_arn
  security_group_id   = local.infra.worker_security_group_id
  execution_role_arn  = local.infra.ecs_task_execution_role_arn
  task_role_arn       = local.infra.ecs_task_app_role_arn

  project     = "hexchess"
  environment = "uat"
  aws_region  = "us-east-1"
  app_name    = "consumer"

  desired_task_count = 1
  task_cpu           = 256
  task_memory        = 512

  rollout = var.rollout

  container_image = "938864279852.dkr.ecr.us-east-1.amazonaws.com/releases/hexchess/consumer"
  image_tag       = var.commit_sha

  env_vars = {
    ACTIVE_PROFILE: "test",
    DB_URL: "postgres://db_readwrite@hexchess-sor.cluster-c10fqqu5dr0g.us-east-1.rds.amazonaws.com/hexchess?sslmode=require",
    REDIS_PUBSUB_NODE: "hexchess-pubsub.hl6d8h.0001.use1.cache.amazonaws.com:6379",
    REDIS_SOR_NODES: join(",", [
      "hexchess-primary-cluster-0001-001.hexchess-primary-cluster.hl6d8h.memorydb.us-east-1.amazonaws.com:6379",
      "hexchess-primary-cluster-0001-002.hexchess-primary-cluster.hl6d8h.memorydb.us-east-1.amazonaws.com:6379",
      "hexchess-primary-cluster-0002-001.hexchess-primary-cluster.hl6d8h.memorydb.us-east-1.amazonaws.com:6379",
      "hexchess-primary-cluster-0002-002.hexchess-primary-cluster.hl6d8h.memorydb.us-east-1.amazonaws.com:6379"
    ])
    # OTEL_EXPORTER_OTLP_ENDPOINT: "localhost:3100"
  }
}