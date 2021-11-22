# CodePipeline Creator using the AWS CDK for Go

_NB: This is being developed right now so it's not finished yet :)_

This code creates an AWS CodePipeline by using Go CDK packaged into a Docker 
container meaning that it can be executed anywhere without `cdk` or even the
Go runtime or tools. The pipeline itself is defined in a JSON file such as:
```
{
    "pipelineName": "YOUR_PIPELINE_NAME",
    "defaultBuildImage": "arn:aws:ecr:AWS_REGION:AWS_ACCOUNT_NUMBER_FOR_IMAGE:repository/container-image",
    "sourceRepo": "YOUR_GITHUB_REPO",
    "accountId": "TARGET_AWS_ACCOUNT_NUMBER",
    "codeStarConnectionArn": "arn:aws:codestar-connections:AWS_REGION:TARGET_AWS_ACCOUNT_NUMBER:connection/AWS_CODESTAR_CONNECTION_HASH",
    "serviceRole": "arn:aws:iam::TARGET_AWS_ACCOUNT_NUMBER:role/service-role/aws-codestar-service-role",
    "workflow": [
      { "stageName": "build", "buildspec": "buildspec.yml", "outputName": "BuildOutput" },
      { "stageName": "test", "buildspec": "buildspec-test.yml", "type": "test" },
      { "stageName": "publish","buildspec": "buildspec-publish.yml", "outputName": "PublishOutput", "buildImageOverride": "arn:aws:ecr:eu-west-1:470820891875:repository/bbc-centos7-ci"},
      { "stageName": "deploy", "buildspec": "buildspec-deploy.yml", "buildImageOverride": "arn:aws:ecr:eu-west-1:470820891875:repository/bbc-centos7-ci"}
    ]
  }
  ```
Defined in the `workflow` are the stages which define:
- `stageName` - The name of the stage.
- `buildspec` - Which file to use as that stage's build file.
- `outputName` (optional) - The output artifact name.
- `buildImageOverride` (optional) - The specific container image to use for this stage.
- `type` (optional) - The pipeline type, `test`/`build`.

See also the [Requirements](Requirements) section below.

## Building
```
docker build -t cdk-pipeline-go .
```

This uses a two-stage Docker build with the first image based on `golang:1.17` building a distributable binary and the second stage based on `alpine:latest` to distribute the tool. This container has an entrypoint of the AWS `cdk` tool, meaning that the `cdk` part of the command `cdk synth -c...` is inferred, making it `synth -c...`.

The pipeline definition is passed to the tool by using the volume mount:
```
`-v /PATH/TO/PIPELINE/DEFINITION/:/var/`
```
...and then passed to the CDK context with:
```
-c pipeline-definition-file=/var/pipeline-definition.json
```

Credentials such as `AWS_ACCESS_KEY_ID` are environment variables.

## Running

### Synth
```
docker run \
  -v /Users/anderb08/workspace/cdk-codepipeline-go/:/var/ \
  cdk-pipeline-go \
  synth -c pipeline-definition-file=/var/pipeline-definition.json
```

### Deploy
```
docker run -e AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} \
    -e AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY} \
    -e AWS_SECRET_KEY_ID=${AWS_SECRET_KEY_ID} \
    -e AWS_SESSION_TOKEN=${AWS_SESSION_TOKEN} \
    -v /PATH/TO/PIPELINE/DEFINITION/:/var/ \
    cdk-pipeline-go \
    deploy -c pipeline-definition-file=/var/pipeline-definition.json
```

### Destroy
```
docker run -e AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} \
    -e AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY} \
    -e AWS_SECRET_KEY_ID=${AWS_SECRET_KEY_ID} \
    -e AWS_SESSION_TOKEN=${AWS_SESSION_TOKEN} \
    -v /PATH/TO/PIPELINE/DEFINITION/:/var/ \
    cdk-pipeline-go \
    destroy -c pipeline-definition-file=/var/pipeline-definition.json
```

## Testing

Run unit tests with `go test`.

## Requirements

### Service Role

The pipeline needs a `serviceRole` which should exist in your AWS account with the trust relationship:

```
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "codebuild.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
```

### CodeStar Connection

The pipeline needs a connection which can be created via the AWS Developer Tools [Create a connection](https://eu-west-1.console.aws.amazon.com/codesuite/settings/connections/create?origin=settings&region=eu-west-1) page which will allow you to connect to your GitHub account. The ARN should be used in the pipeline definition file.
