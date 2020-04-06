package ec2

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/client"

	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/jenkins-x/jx/pkg/cluster"
	"github.com/jenkins-x/jx/pkg/log"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"

	"github.com/pkg/errors"
)

// ec2APIHandler contains some functions to interact with and serves as an abstraction of the EC2 API.
type ec2APIHandler struct {
	ec2 ec2iface.EC2API
}

// NewEC2APIHandler will return an ec2APIHandler with configured credentials
func NewEC2APIHandler(awsSession client.ConfigProvider, ec2api ...ec2iface.EC2API) (*ec2APIHandler, error) {
	if len(ec2api) == 1 {
		return &ec2APIHandler{
			ec2: ec2api[0],
		}, nil
	}
	return &ec2APIHandler{
		ec2: ec2.New(awsSession),
	}, nil
}

// DeleteVolumesForCluster should delete every volume with Kubernetes / JX owned tags
func (e ec2APIHandler) DeleteVolumesForCluster(cluster *cluster.Cluster) error {
	log.Logger().Infof("Attempting to delete jx EC2 Volumes for cluster %s", cluster.Name)
	output, err := e.ec2.DescribeVolumes(&ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String(fmt.Sprintf("tag:kubernetes.io/cluster/%s", cluster.Name)),
				Values: []*string{aws.String("owned")},
			},
			{
				Name:   aws.String("tag:kubernetes.io/created-for/pvc/namespace"),
				Values: []*string{aws.String("jx")},
			},
		},
	})
	if err != nil {
		return errors.Wrapf(err, "error describing volumes for cluster %s", cluster.Name)
	}

	for _, v := range output.Volumes {
		log.Logger().Debugf("Deleting EC2 Volume %s for cluster %s", *v.VolumeId, cluster.Name)
		var attachments []*string
		for _, i := range v.Attachments {
			if *i.State == ec2.VolumeAttachmentStateAttached {
				attachments = append(attachments, i.InstanceId)
			}
		}
		if *v.State == ec2.VolumeStateInUse {
			log.Logger().Debugf("Terminating instances before deleting volumes: %s", strings.Join(aws.StringValueSlice(attachments), ","))
			_, err := e.ec2.TerminateInstances(&ec2.TerminateInstancesInput{
				InstanceIds: attachments,
			})
			if err != nil {
				return errors.Wrapf(err, "error terminating EC2 instances %s",
					strings.Join(aws.StringValueSlice(attachments), ","))
			}
			log.Logger().Info("Waiting until EC2 instances are terminated")
			err = e.ec2.WaitUntilInstanceTerminated(&ec2.DescribeInstancesInput{
				InstanceIds: attachments,
			})
			if err != nil {
				return errors.Wrap(err, "error waiting until EC2 instances are terminated")
			}

			log.Logger().Info("Waiting until Volumes are in an Available state before deleting")
			err = e.ec2.WaitUntilVolumeAvailable(&ec2.DescribeVolumesInput{
				VolumeIds: []*string{v.VolumeId},
			})
			if err != nil {
				return errors.Wrap(err, "error waiting until volume is detached")
			}
		}
		log.Logger().Infof("Deleting volume %s", *v.VolumeId)
		_, err = e.ec2.DeleteVolume(&ec2.DeleteVolumeInput{
			VolumeId: v.VolumeId,
		})
		if err != nil {
			return errors.Wrapf(err, "error deleting EC2 Volume %s", *v.VolumeId)
		}
	}
	return nil
}
