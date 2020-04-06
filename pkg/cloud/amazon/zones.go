package amazon

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/jenkins-x/jx/pkg/cloud/amazon/session"
)

func AvailabilityZones() ([]string, error) {
	answer := []string{}

	sess, err := session.NewAwsSessionWithoutOptions()
	if err != nil {
		return answer, err
	}

	svc := ec2.New(sess)
	input := &ec2.DescribeAvailabilityZonesInput{}

	result, err := svc.DescribeAvailabilityZones(input)
	if err != nil {
		return answer, err
	}
	for _, zone := range result.AvailabilityZones {
		if zone != nil && zone.ZoneName != nil {
			answer = append(answer, *zone.ZoneName)
		}
	}
	return answer, nil
}
