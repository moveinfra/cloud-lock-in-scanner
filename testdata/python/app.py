import boto3
from google.cloud import bigquery, storage  # analytics export
from azure.cosmos import CosmosClient

s3 = boto3.client("s3")
table = boto3.resource("dynamodb").Table("sessions")
kinesis = boto3.client(service_name="kinesis")
ses = boto3.Session().client("sesv2")
