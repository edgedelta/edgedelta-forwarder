module github.com/edgedelta/edgedelta-forwarder

go 1.19

require (
	github.com/aws/aws-lambda-go v1.47.0
	github.com/aws/aws-sdk-go-v2 v1.21.0
	github.com/aws/aws-sdk-go-v2/config v1.18.41
	github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi v1.16.0
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/google/go-cmp v0.5.8
)

require github.com/jmespath/go-jmespath v0.4.0 // indirect

require (
	github.com/aws/aws-sdk-go v1.45.22
	github.com/aws/aws-sdk-go-v2/credentials v1.13.39 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.13.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.41 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.35 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.42 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.35 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.14.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.17.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.22.0 // indirect
	github.com/aws/smithy-go v1.14.2 // indirect
	golang.org/x/sys v0.12.0
)
