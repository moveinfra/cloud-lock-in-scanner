import { SQSClient, SendMessageCommand } from "@aws-sdk/client-sqs";
import { DynamoDBDocumentClient } from "@aws-sdk/lib-dynamodb";
import { Firestore } from "@google-cloud/firestore";
import { BlobServiceClient } from "@azure/storage-blob";

export const sqs = new SQSClient({});
