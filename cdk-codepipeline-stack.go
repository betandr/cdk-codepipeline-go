package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/aws/aws-cdk-go/awscdk"
	"github.com/aws/aws-cdk-go/awscdk/awscodebuild"
	"github.com/aws/aws-cdk-go/awscdk/awscodepipeline"
	"github.com/aws/aws-cdk-go/awscdk/awscodepipelineactions"
	"github.com/aws/aws-cdk-go/awscdk/awsecr"
	"github.com/aws/aws-cdk-go/awscdk/awsiam"
	"github.com/aws/constructs-go/constructs/v3"
	"github.com/aws/jsii-runtime-go"
)

type ConvoyPipelineStackProps struct {
	awscdk.StackProps
}

// PipelineDefinition represents a pipeline with a number
// of WorkflowStage stages in a Workflow
type PipelineDefinition struct {
	PipelineName          string          `json:"pipelineName"`
	DefaultBuildImage     string          `json:"defaultBuildImage"`
	SourceRepo            string          `json:"sourceRepo"`
	AccountId             string          `json:"accountId"`
	CodeStarConnectionArn string          `json:"codeStarConnectionArn"`
	ServiceRole           string          `json:"serviceRole"`
	Workflow              []WorkflowStage `json:"workflow"`
}

// WorkflowStage represents a single pipeline stage
type WorkflowStage struct {
	StageName          string `json:"stageName"`
	Buildspec          string `json:"buildspec"`
	OutputName         string `json:"outputName,omitempty"`
	BuildImageOverride string `json:"buildImageOverride,omitempty"`
	Type               string `json:"type,omitempty"`
}

var pd = PipelineDefinition{}

func NewConvoyPipelineStack(scope constructs.Construct, id string, props *ConvoyPipelineStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// define stack

	pipeline := awscodepipeline.NewPipeline(stack, &id, &awscodepipeline.PipelineProps{
		CrossAccountKeys: jsii.Bool(false),
		PipelineName:     jsii.String(pd.PipelineName),
	})

	serviceRole := awsiam.Role_FromRoleArn(
		stack,
		jsii.String(fmt.Sprintf("cdkbuild-role-%s", pd.PipelineName)),
		jsii.String(pd.ServiceRole),
		&awsiam.FromRoleArnOptions{},
	)

	codeStarConnAction := awscodepipelineactions.NewCodeStarConnectionsSourceAction(
		&awscodepipelineactions.CodeStarConnectionsSourceActionProps{
			ConnectionArn:        &pd.CodeStarConnectionArn,
			ActionName:           jsii.String("SourceAction"),
			Output:               awscodepipeline.NewArtifact(jsii.String("SourceOutput")),
			Owner:                jsii.String("BBC"), // TODO add to pipeline def
			Repo:                 &pd.SourceRepo,
			Branch:               jsii.String("main"), // TODO add to pipeline def
			CodeBuildCloneOutput: jsii.Bool(true)})

	var sourceStageActions []awscodepipeline.IAction
	sourceStageActions = append(sourceStageActions, codeStarConnAction)

	// Source Stage
	pipeline.AddStage(&awscodepipeline.StageOptions{
		StageName: jsii.String("Source"),
		Actions:   &sourceStageActions,
	})

	var buildStageActions []awscodepipeline.IAction

	// Loop through pipeline stages
	for _, wf := range pd.Workflow {

		var buildImage awscodebuild.IBuildImage

		if len(wf.BuildImageOverride) > 0 {
			buildImage = awscodebuild.LinuxBuildImage_FromEcrRepository(
				awsecr.Repository_FromRepositoryArn(
					stack,
					jsii.String(fmt.Sprintf("ecr-%d%s", time.Now().Unix(), wf.StageName)), // TODO fix this labelling issue
					jsii.String(wf.BuildImageOverride)),
				jsii.String("latest"),
			)
		} else {
			buildImage = awscodebuild.LinuxBuildImage_FromEcrRepository(
				awsecr.Repository_FromRepositoryArn(
					stack,
					jsii.String(fmt.Sprintf("defaultEcr_%s", wf.StageName)), // TODO fix this labelling issue
					jsii.String(pd.DefaultBuildImage)),
				jsii.String("latest"),
			)
		}

		// project
		cdkBuild := awscodebuild.NewPipelineProject(
			stack,
			jsii.String(fmt.Sprintf("CdkBuild%s", *jsii.String(wf.StageName))),
			&awscodebuild.PipelineProjectProps{
				BuildSpec: awscodebuild.BuildSpec_FromSourceFilename(jsii.String(wf.Buildspec)),
				Environment: &awscodebuild.BuildEnvironment{
					BuildImage: buildImage,
				},
				Role: serviceRole,
			},
		)

		var outputs []awscodepipeline.Artifact
		var buildType awscodepipelineactions.CodeBuildActionType

		if wf.Type == "test" {
			buildType = awscodepipelineactions.CodeBuildActionType_TEST
		} else {
			buildType = awscodepipelineactions.CodeBuildActionType_BUILD
		}

		codeBuildAction := awscodepipelineactions.NewCodeBuildAction(
			&awscodepipelineactions.CodeBuildActionProps{
				ActionName:  jsii.String(fmt.Sprintf("Build_%s_Action", wf.StageName)),
				Input:       awscodepipeline.NewArtifact(jsii.String("SourceOutput")),
				Project:     cdkBuild,
				Outputs:     &outputs,
				ExtraInputs: &outputs,
				Type:        buildType,
			})

		if len(wf.OutputName) > 0 {
			outputs = append(outputs, awscodepipeline.NewArtifact(jsii.String(wf.OutputName)))
		}

		buildStageActions = append(buildStageActions, codeBuildAction)
	}

	pipeline.AddStage(&awscodepipeline.StageOptions{
		StageName: jsii.String("Build"),
		Actions:   &buildStageActions,
	})

	return stack
}

func main() {
	flag.Parse()
	app := awscdk.NewApp(nil)

	defFile, ok := app.Node().TryGetContext(jsii.String("pipeline-definition-file")).(string)
	if !ok {
		log.Fatal(fmt.Errorf("error: cannot obtain pipeline definition file from context"))
	}

	jsonFile, err := os.Open(defFile)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Reading pipeline definition from %s\n", defFile)

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(byteValue, &pd)
	if err != nil {
		log.Fatal(err)
	}

	pipelineDefinitionJSON, err := json.MarshalIndent(pd, "", "    ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Read pipeline definition as: %s\n", pipelineDefinitionJSON)

	NewConvoyPipelineStack(app, "ConvoyPipelineStack", &ConvoyPipelineStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env() *awscdk.Environment {
	// If unspecified, this stack will be "environment-agnostic".
	// Account/Region-dependent features and context lookups will not work, but a
	// single synthesized template can be deployed anywhere.
	//---------------------------------------------------------------------------
	return nil

	// Uncomment if you know exactly what account and region you want to deploy
	// the stack to. This is the recommendation for production stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String("123456789012"),
	//  Region:  jsii.String("us-east-1"),
	// }

	// Uncomment to specialize this stack for the AWS Account and Region that are
	// implied by the current CLI configuration. This is recommended for dev
	// stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
	//  Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	// }
}
