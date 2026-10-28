const AWS = require("aws-sdk");
const { Kinesis } = require("aws-sdk");

AWS.config.update({ region: "eu-central-1" });
const sns = new AWS.SNS();
const docs = new AWS.DynamoDB.DocumentClient();
const creds = new AWS.SharedIniFileCredentials({ profile: "dev" });
