package amazon

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/cloud/amazon/session"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"k8s.io/apimachinery/pkg/util/uuid"
)

// RegisterAwsCustomDomain registers a wildcard ALIAS for the custom domain
// to point at the given ELB host name
func RegisterAwsCustomDomain(customDomain string, elbAddress string) error {
	sess, err := session.NewAwsSessionWithoutOptions()
	if err != nil {
		return err
	}
	svc := route53.New(sess)

	// find the hosted zone for the domain name
	var hostedZoneId *string
	listZonesInput := &route53.ListHostedZonesInput{}
	err = svc.ListHostedZonesPages(listZonesInput, func(page *route53.ListHostedZonesOutput, hasNext bool) bool {
		if page != nil {
			customDomainParts := strings.Split(customDomain, ".")
			for _, r := range page.HostedZones {
				strings.Split(customDomain, ".")
				if r != nil && r.Name != nil && (*r.Name == customDomain || *r.Name == customDomain+"." || *r.Name == strings.Join(customDomainParts[1:], ".")+".") {
					hostedZoneId = r.Id
					return false
				}
			}
		}
		return true
	})
	if err != nil {
		return err
	}

	if hostedZoneId == nil {
		// lets create the hosted zone!
		callerRef := string(uuid.NewUUID())
		createInput := &route53.CreateHostedZoneInput{
			Name:            aws.String(customDomain),
			CallerReference: aws.String(callerRef),
		}
		results, err := svc.CreateHostedZone(createInput)
		if err != nil {
			return err
		}
		if results.HostedZone == nil {
			return fmt.Errorf("No HostedZone created for name %s!", customDomain)
		}

		hostedZoneId = results.HostedZone.Id
		if hostedZoneId == nil {
			return fmt.Errorf("No HostedZone ID created for name %s!", customDomain)
		}
	}

	upsert := route53.ChangeActionUpsert
	ttl := int64(300)
	recordType := "CNAME"
	wildcard := "*." + customDomain
	info := util.ColorInfo
	log.Logger().Infof("About to insert/update DNS %s record into HostedZone %s with wildcard %s pointing to %s", info(recordType), info(*hostedZoneId), info(wildcard), info(elbAddress))

	changeInput := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: hostedZoneId,
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action: &upsert,
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name: aws.String(wildcard),
						Type: aws.String(recordType),
						TTL:  &ttl,
						ResourceRecords: []*route53.ResourceRecord{
							{
								Value: aws.String(elbAddress),
							},
						},
					},
				},
			},
		},
	}
	_, err = svc.ChangeResourceRecordSets(changeInput)
	if err != nil {
		return fmt.Errorf("Failed to update record for hostedZoneID %s: %s", *hostedZoneId, err)
	}
	log.Logger().Infof("Updated HostZone ID %s successfully", info(*hostedZoneId))
	return nil
}
