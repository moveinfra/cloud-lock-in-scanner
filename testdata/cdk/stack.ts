import { Stack, StackProps, aws_sqs as sqs } from "aws-cdk-lib";
import * as dynamodb from "aws-cdk-lib/aws-dynamodb";
import * as lambda from "aws-cdk-lib/aws-lambda";
import * as ec2 from "aws-cdk-lib/aws-ec2";
import { Construct } from "constructs";

export class OrdersStack extends Stack {
  constructor(scope: Construct, id: string, props?: StackProps) {
    super(scope, id, props);
    new dynamodb.Table(this, "Orders", { partitionKey: { name: "id", type: dynamodb.AttributeType.STRING } });
  }
}
