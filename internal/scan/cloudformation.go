package scan

import (
	"regexp"
	"strings"

	"github.com/moveinfra/cloud-lock-in-scanner/internal/rules"
)

// cfnType matches `Type: AWS::X::Y` in YAML (quoted or not) and `"Type": "AWS::X::Y"` in JSON. Matching text
// instead of parsing YAML keeps short-form tags like !Ref, !GetAtt and !Sub from getting in the way.
var cfnType = regexp.MustCompile(`["']?\bType["']?[ \t]*:[ \t]*["']?(AWS::[A-Za-z0-9]+::[A-Za-z0-9]+(?:::[A-Za-z0-9]+)?)`)

func cloudFormation(src string, lines lineIndex) []Hit {
	if !strings.Contains(src, "AWS::") {
		return nil
	}
	var hits []Hit
	for _, m := range cfnType.FindAllStringSubmatchIndex(src, -1) {
		typ := src[m[2]:m[3]]
		source := "cloudformation"
		if strings.HasPrefix(typ, "AWS::Serverless::") {
			source = "sam"
		}
		hits = append(hits, Hit{
			Cloud: "aws", Kind: rules.KindCloudFormation, Name: typ, Source: source, Text: typ,
			Line: lines.line(m[2]),
		})
	}
	return hits
}

var (
	slsProviderShort = regexp.MustCompile(`(?m)^provider:[ \t]*["']?([A-Za-z]+)`)
	slsProviderName  = regexp.MustCompile(`(?m)^provider:[ \t]*\n(?:(?:[ \t]+.*|[ \t]*)\n)*?[ \t]+name:[ \t]*["']?([A-Za-z]+)`)
	slsFunctions     = regexp.MustCompile(`(?m)^functions:`)
	slsEvent         = regexp.MustCompile(`(?m)^[ \t]*-[ \t]*(httpApi|http|websocket|sqs|sns|stream|s3|schedule|eventBridge|cloudwatchEvent|cloudwatchLog|cognitoUserPool|alb|iot|msk|activemq|rabbitmq|kinesis)[ \t]*:`)
)

// slsEvents maps a Serverless Framework event type to the AWS SDK service name it relies on.
var slsEvents = map[string]string{
	"http": "apigateway", "httpApi": "apigatewayv2", "websocket": "apigatewayv2",
	"sqs": "sqs", "sns": "sns", "s3": "s3", "kinesis": "kinesis",
	"schedule": "eventbridge", "eventBridge": "eventbridge", "cloudwatchEvent": "eventbridge",
	"cloudwatchLog": "logs", "cognitoUserPool": "cognitoidp", "alb": "elasticloadbalancingv2",
	"iot": "iot", "msk": "kafka", "activemq": "mq", "rabbitmq": "mq",
}

// slsFunctionsByProvider maps a Serverless Framework provider to the cloud and SDK name of its functions service.
var slsFunctionsByProvider = map[string][2]string{
	"aws":    {"aws", "lambda"},
	"google": {"gcp", "functions"},
	"azure":  {"azure", "functions"},
}

func serverless(src string, lines lineIndex) []Hit {
	provider := "aws"
	if m := slsProviderName.FindStringSubmatch(src); m != nil {
		provider = strings.ToLower(m[1])
	} else if m := slsProviderShort.FindStringSubmatch(src); m != nil {
		provider = strings.ToLower(m[1])
	}
	fn, ok := slsFunctionsByProvider[provider]
	if !ok {
		return nil
	}
	var hits []Hit
	if m := slsFunctions.FindStringIndex(src); m != nil {
		hits = append(hits, Hit{
			Cloud: fn[0], Kind: rules.KindSDK, Name: fn[1], Source: "serverless functions",
			Text: "functions", Line: lines.line(m[0]),
		})
	}
	if provider != "aws" {
		return hits
	}
	for _, m := range slsEvent.FindAllStringSubmatchIndex(src, -1) {
		ev := src[m[2]:m[3]]
		name := slsEvents[ev]
		if ev == "stream" {
			// A stream event is DynamoDB unless the ARN or type on the next lines says Kinesis.
			name = "dynamodbstreams"
			next := src[m[1]:min(len(src), m[1]+300)]
			if i := strings.Index(next, "\n  -"); i >= 0 {
				next = next[:i]
			}
			if strings.Contains(strings.ToLower(next), "kinesis") {
				name = "kinesis"
			}
		}
		hits = append(hits, Hit{
			Cloud: "aws", Kind: rules.KindSDK, Name: name, Source: "serverless event", Text: ev,
			Line: lines.line(m[2]),
		})
	}
	return hits
}
