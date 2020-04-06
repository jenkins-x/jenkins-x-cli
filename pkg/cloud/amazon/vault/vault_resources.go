package vault

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/request"

	"github.com/aws/aws-sdk-go/service/iam"

	uuid "github.com/satori/go.uuid"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	goformation "github.com/awslabs/goformation/cloudformation"
	"github.com/awslabs/goformation/cloudformation/resources"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

const stackNamePrefix = "jenkins-x-vault-stack"

const (
	readCapacityUnits               = 2
	writeCapacityUnits              = 2
	vaultCloudFormationTemplateName = "vault_cf_tmpl.yml"
	resourceSuffixParamName         = "ResourcesSuffixParameter"
	iamUserParamName                = "IAMUser"
	dynamoDBTableNameParamName      = "DynamoDBTableName"
	s3BucketNameParamName           = "S3BucketName"
)

// WaitConfigFunc function that configures a waiter for an AWS request with a 10 minute timeout
func WaitConfigFunc(waiter *request.Waiter) {
	waiter.Delay = request.ConstantWaiterDelay(30 * time.Second)
	waiter.MaxAttempts = 20
}

// ResourceCreationOpts The input parameters to create a vault by default on aws
type ResourceCreationOpts struct {
	Region          string
	Domain          string
	Username        string
	TableName       string
	BucketName      string
	AWSTemplatesDir string
	UniqueSuffix    string
	AccessKeyID     string
	SecretAccessKey string
}

// StackOutputs the CloudFormation stack outputs for Vault
type StackOutputs struct {
	KMSKeyARN        *string
	S3BucketARN      *string
	DynamoDBTableARN *string
}

// CreateVaultResources will automatically create the prerequisites for vault on aws
// Deprecated
// Will be deleted once we don't support jx install
func CreateVaultResources(vaultParams ResourceCreationOpts) (*string, *string, *string, *string, *string, error) {
	log.Logger().Infof("Creating vault prerequisite resources with following values, %s, %s, %s, %s",
		util.ColorInfo(vaultParams.Region),
		util.ColorInfo(vaultParams.Domain),
		util.ColorInfo(vaultParams.Username),
		util.ColorInfo(vaultParams.TableName))

	valueUUID, err := uuid.NewV4()
	if err != nil {
		return nil, nil, nil, nil, nil, errors.Wrapf(err, "generating UUID failed")
	}

	// Create suffix to apply to resources
	suffixString := valueUUID.String()[:7]

	template := goformation.NewTemplate()

	awsDynamoDbKey := "AWSDynamoDBTable"
	dynamoDbTableName := vaultParams.TableName + "_" + suffixString
	dynamoDbTable := createDynamoDbTable(dynamoDbTableName)
	template.Resources[awsDynamoDbKey] = &dynamoDbTable

	awsS3BucketKey := "AWSS3Bucket"
	s3Name, s3Bucket := createS3Bucket(vaultParams.Region, vaultParams.Domain, suffixString)
	template.Resources[awsS3BucketKey] = s3Bucket

	awsIamUserKey := "AWSIAMUser"
	iamUsername := vaultParams.Username + suffixString
	iamUser := createIamUser(iamUsername)
	template.Resources[awsIamUserKey] = &iamUser

	awsKmsKey := "AWSKMSKey"
	kmsKey, err := createKmsKey(iamUsername, []string{awsIamUserKey})
	if err != nil {
		return nil, nil, nil, nil, nil, errors.Wrapf(err, "generating the vault CloudFormation template failed")
	}
	template.Resources[awsKmsKey] = kmsKey

	awsIamPolicy := "AWSIAMPolicy"
	policy := createIamUserPolicy(iamUsername, []string{awsDynamoDbKey, awsS3BucketKey, awsKmsKey, awsIamUserKey})
	template.Resources[awsIamPolicy] = &policy

	log.Logger().Debugf("Generating the vault CloudFormation template")

	// and also the YAML AWS CloudFormation template
	yaml, err := template.JSON()
	if err != nil {
		return nil, nil, nil, nil, nil, errors.Wrapf(err, "generating the vault CloudFormation template failed")
	}

	log.Logger().Debugf("Generated the vault CloudFormation template successfully")

	yamlProcessed := string(yaml)
	yamlProcessed = setProperIntrinsics(yamlProcessed)

	// Create dynamic stack name
	stackName := stackNamePrefix + suffixString

	err = runCloudFormationTemplate(&yamlProcessed, &stackName, nil)
	if err != nil {
		return nil, nil, nil, nil, nil, errors.Wrap(err, "there was a problem running the Vault CloudFormation stack")
	}

	kmsID, err := getKmsID(awsKmsKey, stackName)
	if err != nil {
		return nil, nil, nil, nil, nil, errors.Wrapf(err, "generating the vault CloudFormation template failed")
	}

	accessKey, keySecret, err := createAccessKey(iamUsername)
	if err != nil {
		return nil, nil, nil, nil, nil, errors.Wrapf(err, "generating the vault CloudFormation template failed")
	}

	return accessKey, keySecret, kmsID, s3Name, &dynamoDbTableName, nil
}

// CreateVaultResourcesBoot creates required Vault resources in AWS for a boot cluster
func CreateVaultResourcesBoot(vaultParams ResourceCreationOpts) (*string, *string, *string, *string, *string, error) {
	log.Logger().Infof("Creating vault resources with following values, %s, %s, %s, %s",
		util.ColorInfo(vaultParams.Region),
		util.ColorInfo(vaultParams.Username),
		util.ColorInfo(vaultParams.TableName),
		util.ColorInfo(vaultParams.BucketName))

	templatePath := filepath.Join(vaultParams.AWSTemplatesDir, vaultCloudFormationTemplateName)
	log.Logger().Debugf("Attempting to read Vault CloudFormation template from path %s", templatePath)
	exists, err := util.FileExists(templatePath)
	if err != nil {
		return nil, nil, nil, nil, nil, errors.Wrap(err, "there was a problem loading the vault_cf_tmpl.yml file")
	} else if !exists {
		return nil, nil, nil, nil, nil, fmt.Errorf("vault cloud formation template %s doesn't exist", templatePath)
	}
	templateBytes, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	stackName := stackNamePrefix + vaultParams.UniqueSuffix
	err = runCloudFormationTemplate(aws.String(string(templateBytes)), aws.String(stackName), []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(resourceSuffixParamName),
			ParameterValue: aws.String(vaultParams.UniqueSuffix),
		},
		{
			ParameterKey:   aws.String(iamUserParamName),
			ParameterValue: aws.String(vaultParams.Username),
		},
		{
			ParameterKey:   aws.String(dynamoDBTableNameParamName),
			ParameterValue: aws.String(vaultParams.TableName),
		},
		{
			ParameterKey:   aws.String(s3BucketNameParamName),
			ParameterValue: aws.String(vaultParams.BucketName),
		},
	})
	if err != nil {
		return nil, nil, nil, nil, nil, errors.Wrapf(err, "executing the Vault CloudFormation ")
	}

	vaultStackOutputs, err := extractVaultStackOutputs(stackName)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	if vaultParams.AccessKeyID == "" || vaultParams.SecretAccessKey == "" {
		log.Logger().Info("Creating secret access keys")
		accessKey, keySecret, err := createAccessKey(vaultParams.Username)
		if err != nil {
			return nil, nil, nil, nil, nil, errors.Wrapf(err, "generating the vault CloudFormation template failed")
		}
		vaultParams.AccessKeyID = *accessKey
		vaultParams.SecretAccessKey = *keySecret
	}

	return aws.String(vaultParams.AccessKeyID), aws.String(vaultParams.SecretAccessKey), vaultStackOutputs.KMSKeyARN, vaultStackOutputs.S3BucketARN, vaultStackOutputs.DynamoDBTableARN, nil
}

/*
	Currently there was is no way to bypass the intrinsic functions
	If Fn::Sub (with double colon) is used it is evaluated and removed from the templat output
	However the evaluation did not seem to work and evaluated ${Name} based values back to "${Name}"
	As a workaround Fn:Sub is used instead.
	Reference : https://github.com/awslabs/goformation/issues/62

	Add any more intrinsics here if required.
*/
func setProperIntrinsics(yamlTemplate string) string {
	var yamlProcessed string
	yamlProcessed = strings.Replace(yamlTemplate, "Fn:Sub", "Fn::Sub", -1)
	return yamlProcessed
}

func getKmsID(kmsOutputName string, stackName string) (*string, error) {
	log.Logger().Debugf("Retrieving the vault kms id")
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	svc := cloudformation.New(sess)

	desInput := &cloudformation.DescribeStackResourceInput{
		LogicalResourceId: aws.String(kmsOutputName),
		StackName:         aws.String(stackName),
	}
	output, err := svc.DescribeStackResource(desInput)

	if err != nil {
		return nil, errors.Wrapf(err, "unable to retrieve kms Id")
	}

	log.Logger().Debugf("Retrieved the vault kms id successfully")
	return output.StackResourceDetail.PhysicalResourceId, nil
}

func createDynamoDbTable(tableName string) resources.AWSDynamoDBTable {
	provisionedThroughput := &resources.AWSDynamoDBTable_ProvisionedThroughput{
		ReadCapacityUnits:  readCapacityUnits,
		WriteCapacityUnits: writeCapacityUnits,
	}

	keyList := []resources.AWSDynamoDBTable_KeySchema{
		{
			AttributeName: "Path",
			KeyType:       "HASH",
		},
		{
			AttributeName: "Key",
			KeyType:       "RANGE",
		},
	}

	attributeDefinitions := []resources.AWSDynamoDBTable_AttributeDefinition{
		{
			AttributeName: "Path",
			AttributeType: "S",
		},
		{
			AttributeName: "Key",
			AttributeType: "S",
		},
	}

	configuation := resources.AWSDynamoDBTable{
		TableName:             tableName,
		ProvisionedThroughput: provisionedThroughput,
		KeySchema:             keyList,
		AttributeDefinitions:  attributeDefinitions,
		Tags: []resources.Tag{
			{
				Key:   "Name",
				Value: "vault-dynamo-db-table",
			},
		},
	}

	return configuation
}

func createS3Bucket(region string, domain string, suffixString string) (*string, *resources.AWSS3Bucket) {
	bucketName := "vault-unseal." + region + "." + domain + "." + suffixString
	log.Logger().Debugf(bucketName)

	bucketConfig := resources.AWSS3Bucket{
		AccessControl: "Private",
		BucketName:    bucketName,
		VersioningConfiguration: &resources.AWSS3Bucket_VersioningConfiguration{
			Status: "Suspended",
		},
	}

	return &bucketName, &bucketConfig
}

func createKmsKey(username string, depends []string) (*resources.AWSKMSKey, error) {
	type FnSubWrapper struct {
		FnSub string `json:"Fn:Sub"`
	}

	type Principal struct {
		AWS []FnSubWrapper
	}

	type PolicyDocument struct {
		Sid       string
		Effect    string
		Action    string
		Resource  string
		Principal Principal
	}

	type PolicyRoot struct {
		Version   string
		Statement []PolicyDocument
	}

	document := PolicyRoot{
		Version: "2012-10-17",
		Statement: []PolicyDocument{
			{
				Sid:    "Enable IAM User Permissions",
				Effect: "Allow",
				Principal: Principal{
					AWS: []FnSubWrapper{
						{
							FnSub: "arn:aws:iam::${AWS::AccountId}:root",
						},
						{
							FnSub: "arn:aws:iam::${AWS::AccountId}:user/" + username,
						},
					},
				},
				Action:   "kms:*",
				Resource: "*",
			},
		},
	}

	kmsKey := resources.AWSKMSKey{
		Description: "KMS Key for bank vault unseal",
		KeyPolicy:   document,
	}

	kmsKey.SetDependsOn(depends)

	return &kmsKey, nil
}

func createIamUser(username string) resources.AWSIAMUser {
	iamUser := resources.AWSIAMUser{
		UserName: username,
	}
	return iamUser
}

func createIamUserPolicy(username string, depends []string) resources.AWSIAMPolicy {
	type FnSubWrapper struct {
		FnSub string `json:"Fn:Sub"`
	}

	type PolicyDocument struct {
		Sid      string
		Effect   string
		Action   []string
		Resource FnSubWrapper
	}

	type PolicyRoot struct {
		Version   string
		Statement []PolicyDocument
	}

	policyName := "vault_" + username

	policyDocument := resources.AWSIAMPolicy{
		PolicyName: policyName,
		Users:      []string{username},
		PolicyDocument: PolicyRoot{
			Version: "2012-10-17",
			Statement: []PolicyDocument{
				{
					Sid:    "DynamoDB",
					Effect: "Allow",
					Action: []string{
						"dynamodb:DescribeLimits",
						"dynamodb:DescribeTimeToLive",
						"dynamodb:ListTagsOfResource",
						"dynamodb:DescribeReservedCapacityOfferings",
						"dynamodb:DescribeReservedCapacity",
						"dynamodb:ListTables",
						"dynamodb:BatchGetItem",
						"dynamodb:BatchWriteItem",
						"dynamodb:CreateTable",
						"dynamodb:DeleteItem",
						"dynamodb:GetItem",
						"dynamodb:GetRecords",
						"dynamodb:PutItem",
						"dynamodb:Query",
						"dynamodb:UpdateItem",
						"dynamodb:Scan",
						"dynamodb:DescribeTable",
					},
					Resource: FnSubWrapper{
						FnSub: "arn:aws:dynamodb:${AWS::Region}:${AWS::AccountId}:table/*",
					},
				},
				{
					Sid:    "S3",
					Effect: "Allow",
					Action: []string{
						"s3:PutObject",
						"s3:GetObject",
					},
					Resource: FnSubWrapper{
						FnSub: "${" + depends[1] + ".Arn}/*",
					},
				},
				{
					Sid:    "S3List",
					Effect: "Allow",
					Action: []string{
						"s3:ListBucket",
					},
					Resource: FnSubWrapper{
						FnSub: "${" + depends[1] + ".Arn}",
					},
				},
				{
					Sid:    "KMS",
					Effect: "Allow",
					Action: []string{
						"kms:Encrypt",
						"kms:Decrypt",
					},
					Resource: FnSubWrapper{
						FnSub: "${" + depends[2] + ".Arn}",
					},
				},
			},
		},
	}

	policyDocument.SetDependsOn(depends)

	return policyDocument
}

func runCloudFormationTemplate(templateBody *string, stackName *string, parameters []*cloudformation.Parameter) error {

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	// Create CloudFormation client in region
	svc := cloudformation.New(sess)

	desInput := &cloudformation.DescribeStacksInput{StackName: stackName}

	input := &cloudformation.CreateStackInput{
		TemplateBody: templateBody,
		StackName:    stackName,
		Capabilities: []*string{
			aws.String("CAPABILITY_NAMED_IAM"),
		},
		Parameters: parameters,
	}
	createOutput, err := svc.CreateStack(input)
	if err != nil {
		return errors.Wrapf(err, "unable to create vault prerequisite resources")
	}

	log.Logger().Info("Vault CloudFormation stack created")
	cloudFormationURL := fmt.Sprintf("https://console.aws.amazon.com/cloudformation/home?region=%s#/stacks/stackinfo?stackId=%s", *sess.Config.Region, *createOutput.StackId)
	log.Logger().Infof("You can watch progress in the CloudFormation console: %s", util.ColorInfo(cloudFormationURL))

	// Wait until stack is created
	err = svc.WaitUntilStackCreateCompleteWithContext(aws.BackgroundContext(), desInput, WaitConfigFunc)
	if err != nil {
		return errors.Wrapf(err, "unable to create vault prerequisite resources")
	}
	log.Logger().Debugf("Ran the vault CloudFormation template successfully")
	return nil
}

func extractVaultStackOutputs(stackName string) (*StackOutputs, error) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	// Create CloudFormation client in region
	svc := cloudformation.New(sess)

	desInput := &cloudformation.DescribeStacksInput{StackName: aws.String(stackName)}

	output, err := svc.DescribeStacks(desInput)
	if err != nil {
		return nil, err
	}
	vsp := &StackOutputs{}
	if len(output.Stacks) > 0 {
		stack := output.Stacks[0]
		for _, v := range stack.Outputs {
			switch *v.OutputKey {
			case "AWSS3Bucket":
				vsp.S3BucketARN = v.OutputValue
			case "AWSKMSKey":
				vsp.KMSKeyARN = v.OutputValue
			case "AWSDynamoDBTable":
				vsp.DynamoDBTableARN = v.OutputValue
			default:
				log.Logger().Warnf("CloudFormation parameter %s not expected", *v.OutputKey)
			}
		}
	}
	return vsp, nil
}

/**
We can create an access key using cloudformation, however this results the secret being logged.
There is supposed to be a way using a custom type but for now using this
*/
func createAccessKey(username string) (*string, *string, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, nil, err
	}
	svc := iam.New(sess)
	input := &iam.CreateAccessKeyInput{
		UserName: aws.String(username),
	}
	result, err := svc.CreateAccessKey(input)

	if err != nil {
		return nil, nil, errors.Wrapf(err, "unable to create access key")
	}

	log.Logger().Debugf("Created the vault access key successfully")
	return result.AccessKey.AccessKeyId, result.AccessKey.SecretAccessKey, nil
}
