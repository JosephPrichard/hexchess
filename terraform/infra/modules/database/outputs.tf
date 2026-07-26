output "policy_arn_map" {
  description = "Map of each role name to each policy arn"
  value       = {
    for key, policy in aws_iam_policy.db_role_policy_connect : key => policy.arn
  }
}