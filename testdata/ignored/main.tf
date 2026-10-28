resource "aws_redshift_cluster" "ignored" {
  cluster_identifier = "this file is excluded by .lockinignore"
}
