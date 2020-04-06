// +build unit

package ec2

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/jenkins-x/jx/pkg/cluster"

	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/stretchr/testify/assert"
)

type mockedEC2 struct {
	ec2iface.EC2API
	DescribeVolumesVols    []*ec2.Volume
	DeleteVolumeValidation func([]*ec2.Volume, []*ec2.Volume)
}

func (m mockedEC2) DescribeVolumes(vi *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error) {
	var volumes []*ec2.Volume
	for _, v := range m.DescribeVolumesVols {
		for _, t := range v.Tags {
			if strings.Contains(*vi.Filters[0].Name, *t.Key) {
				volumes = append(volumes, v)
			}
		}
	}
	return &ec2.DescribeVolumesOutput{
		Volumes: volumes,
	}, nil
}

func (m mockedEC2) DeleteVolume(input *ec2.DeleteVolumeInput) (*ec2.DeleteVolumeOutput, error) {
	if len(m.DescribeVolumesVols) > 0 {
		for i, v := range m.DescribeVolumesVols {
			if *v.VolumeId == *input.VolumeId {
				deleted := append(m.DescribeVolumesVols[:i], m.DescribeVolumesVols[i+1:]...)
				m.DeleteVolumeValidation(m.DescribeVolumesVols, deleted)
			}
		}
	}
	return &ec2.DeleteVolumeOutput{}, nil
}

func TestEC2Options_DeleteVolumesForCluster(t *testing.T) {
	sess, err := session.NewSession()
	assert.NoError(t, err)
	ec2Options, err := NewEC2APIHandler(sess, mockedEC2{
		DescribeVolumesVols: []*ec2.Volume{
			{
				State:    aws.String(ec2.VolumeStateAvailable),
				VolumeId: aws.String("vol1"),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("kubernetes.io/cluster/cluster1"),
						Value: aws.String("owner"),
					},
				},
			},
			{
				State:    aws.String(ec2.VolumeStateAvailable),
				VolumeId: aws.String("vol2"),
			},
		},
		DeleteVolumeValidation: func(original []*ec2.Volume, deleted []*ec2.Volume) {
			assert.True(t, len(original) == (len(deleted)+1), "the deleted volume slice should be one element smaller than the original")
		},
	})
	assert.NoError(t, err)
	err = ec2Options.DeleteVolumesForCluster(&cluster.Cluster{
		Name: "cluster1",
	})
	assert.NoError(t, err)
}
