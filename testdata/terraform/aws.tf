# A small AWS stack: networking, a VM, Postgres, and some serverless parts.

resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}

resource "aws_subnet" "private" {
  vpc_id     = aws_vpc.main.id
  cidr_block = "10.0.1.0/24"
}

resource "aws_security_group" "web" {
  vpc_id = aws_vpc.main.id
}

resource "aws_iam_role" "lambda" {
  name               = "orders-lambda"
  assume_role_policy = data.aws_iam_policy_document.assume.json
}

data "aws_iam_policy_document" "assume" {
  statement {
    actions = ["sts:AssumeRole"]
  }
}

resource "aws_instance" "web" {
  ami           = "ami-0123456789"
  instance_type = "t3.medium"
}

resource "aws_db_instance" "orders" {
  engine         = "postgres"
  instance_class = "db.t3.medium"
}

resource "aws_rds_cluster" "reports" {
  engine      = "aurora-postgresql"
  engine_mode = "provisioned"

  serverlessv2_scaling_configuration {
    min_capacity = 0.5
    max_capacity = 4
  }
}

resource "aws_dynamodb_table" "sessions" {
  name     = "sessions"
  hash_key = "id"
}

resource "aws_lambda_function" "orders" {
  function_name = "orders"
  role          = aws_iam_role.lambda.arn
}

resource "aws_sqs_queue" "orders" {
  name = "orders"
}

resource "aws_s3_bucket" "assets" {
  bucket = "example-assets"
}

resource "aws_lb" "web" {
  load_balancer_type = "application"
}

resource "aws_gamelift_fleet" "game" {
  name = "not-in-the-rules"
}

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.0"
}
