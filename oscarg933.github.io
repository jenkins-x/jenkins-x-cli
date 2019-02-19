Skip to content
Your account has been flagged.
Because of that, your profile is hidden from the public. If you believe this is a mistake, contact support to have your account status reviewed.
IP jenkins go
Repositories0
Code106K
Commits11K
Issues5K
Marketplace0
Topics0
Wikis688+
Users0
Language

Sort

106,828 code results
@baselm
baselm/self-healing-latex – auto-scaling.sh
Showing the top four matches
Last indexed on Sep 24, 2018
Shell
      service: 'go-demo_main'
      scale: 'up'
    receiver: 'jenkins-go-demo_main-up'
  - match:
      service: 'go-demo_main'
  - name: 'jenkins-go-demo_main-down'
    webhook_configs:
      - send_resolved: false
        url: 'http://$(docker-machine ip swarm-1)/jenkins/job/service-scale/buildWithParameters?token=DevOps22&service=go-demo_main&scale=-1'
@baselm
baselm/self-healing-latex – # Setup Cluster-autoscaling
Showing the top six matches
Last indexed on Sep 23, 2018
    receiver: 'jenkins-go-demo_main-up'
  - match:
      service: 'go-demo_main'
      scale: 'down'
    receiver: 'jenkins-go-demo_main-down'
open "http://$(docker-machine ip swarm-1)/jenkins/blue/organizations/jenkins/service-scale/activity"

docker stack ps -f desired-state=running go-demo
@adityai
adityai/JenkinsDockerSwarm – README.md
Showing the top six matches
Last indexed on Jul 5, 2018
Markdown
curl "$(docker-machine ip swarm-1):8080/v1/docker-flow-proxy/reconfigure?serviceName=go-demo&servicePath=/demo&port=8080"

curl -i $(docker-machine ip swarm-1)/demo/hello
docker service ps jenkins

# Jenkins setup
open http://$(docker-machine ip swarm-1):8082/jenkins

cat docker/jenkins/secrets/initialAdminPassword
@naviat
naviat/DevOps-Toolkit-2.0 – jenkins-demo.md
Showing the top five matches
Last indexed on Jul 8, 2018
Markdown
ssh -i devops21.pem ubuntu@$(terraform \
  output swarm_manager_1_public_ip)

curl -o jenkins.yml \
exit

open "http://$(terraform output swarm_manager_1_public_ip)/jenkins"
```


# Jenkins Setup

---
@ainet8infosec
ainet8infosec/3.va.GO – Jenksify.sh
Showing the top five matches
Last indexed 10 days ago
Shell
#JENKINS_CRUMB=$(curl -s "http://${MANAGER_IP}:8888/crumbIssuer/api/xml?xpath=concat(//crumbRequestField,\":\",//crumb)" -u ${JENKINS_USER}:${JENKINS_PASS})
     
curl -s -XPOST "${MANAGER_IP}:8888/createItem?name=testCI" \
curl -s -u ${JENKINS_USER}:${JENKINS_PASS} -X POST ${MANAGER_IP}:8888/job/testCI/build

echo "Recreate and Seed the POSTGRES...."
@vfarcic
vfarcic/vfarcic.github.io – jenkins-demo.md
Showing the top six matches
Last indexed on Jun 27, 2018
Markdown
open "http://$CLUSTER_DNS/jenkins"

ssh -i workshop.pem docker@$CLUSTER_IP

docker stack rm jenkins

exit
  output swarm_manager_1_public_ip) docker stack ps go-demo

curl $(terraform output swarm_manager_1_public_ip)/demo/hello
```


# Jenkins Failover

---
@herrphon
herrphon/herrphon.github.io – docker-swarm-jenkins.md
Showing the top four matches
Last indexed on Jun 30, 2018
Markdown
docker service create --name jenkins-agent \
    -e COMMAND_OPTIONS="-master http://$(docker-machine ip swarm-1):8082/jenkins \
docker swarm join --token $TOKEN --advertise-addr $(docker-machine ip swarm-test-2) $(docker-machine ip swarm-test-1):2377
```

Example jenkins pipeline:

``` groovy
  node("docker") {
@vfarcic
vfarcic/vfarcic.github.io – helm-demo.md
Showing the top six matches
Last indexed on Nov 20, 2018
Markdown
# If GKE
ADDR=$(kubectl -n jenkins get svc jenkins \
    -o jsonpath="{.status.loadBalancer.ingress[0].ip}"):8080

# If minikube
ADDR=$(minikube ip):$(kubectl -n jenkins get svc jenkins \
    -o jsonpath="{.spec.ports[0].nodePort}")
@firegnome
firegnome/spring-cloud-prototype-gcp – README.md
Showing the top four matches
Last indexed on Dec 19, 2018
Markdown
Add Gitlab API token:
* Go to `Manage Jenkins > Configure System` and scroll down to `Gitlab` section.
Create new Pipeline:
* Go to `Jenkins > new element`
* Select `Multibranch` Pipeline and click create
* Under `Branch Sources` click `add source > Git`
@thoughtquery
thoughtquery/devops22 – dm-swarm-13.sh
Showing the top four matches
Last indexed on Jul 12, 2018
Shell
      service: 'go-demo_main'
      scale: 'up'
    receiver: 'jenkins-go-demo_main-up'
  - match:
      service: 'go-demo_main'
  - name: 'jenkins-go-demo_main-down'
    webhook_configs:
      - send_resolved: false
        url: 'http://$(docker-machine ip swarm-1)/jenkins/job/service-scale/buildWithParameters?token=DevOps22&service=go-demo_main&scale=-1'
© 2019 GitHub, Inc.
Terms
Privacy
Security
Status
Help
Contact GitHub
Pricing
API
Training
Blog
About
Press h to open a hovercard with more details.
