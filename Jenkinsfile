pipeline {
    agent {
        label "jenkins-go"
    }
    environment {
        CHARTMUSEUM_CREDS   = credentials('jenkins-x-chartmuseum')
        JENKINS_CREDS       = credentials('test-jenkins-user')
        GH_CREDS            = credentials('jenkins-x-github')
        GHE_CREDS           = credentials('ghe-test-user')
        GKE_SA              = credentials('gke-sa')
        BUILD_NUMBER        = "$BUILD_NUMBER"
        GIT_USERNAME        = "$GH_CREDS_USR"
        GIT_API_TOKEN       = "$GH_CREDS_PSW"
        GITHUB_ACCESS_TOKEN = "$GH_CREDS_PSW"

        JOB_NAME            = "$JOB_NAME"
        BRANCH_NAME         = "$BRANCH_NAME"
        ORG                 = 'jenkinsxio'
        APP_NAME            = 'jx'
        PREVIEW_VERSION     = "0.0.0-SNAPSHOT-$BRANCH_NAME-$BUILD_NUMBER"
        TEAM                = "$BRANCH_NAME-$BUILD_NUMBER".toLowerCase()
        PREVIEW_IMAGE_TAG   = "SNAPSHOT-JX-$BRANCH_NAME-$BUILD_NUMBER"

        // for BDD tests
        GIT_PROVIDER_URL     = "https://github.beescloud.com"
        GHE_TOKEN            = "$GHE_CREDS_PSW"

        JX_DISABLE_DELETE_APP  = "true"
        JX_DISABLE_DELETE_REPO = "true"
    }
    stages {
        stage('CI Build and Test') {
            when {
                branch 'PR-*'
            }
            steps {
                dir ('/home/jenkins/go/src/github.com/jenkins-x/jx') {
                    checkout scm
                    container('go') {
                        sh "make linux"
                        sh "make test"
                        sh "./build/linux/jx --help"

                        sh "docker build -t docker.io/$ORG/$APP_NAME:$PREVIEW_VERSION ."

                        sh "make preview"

                        // lets create a team for this PR and run the BDD tests
                        sh "gcloud auth activate-service-account --key-file $GKE_SA"
                        sh "gcloud container clusters get-credentials anthorse --zone europe-west1-b --project jenkinsx-dev"


                        sh "sed 's/\$VERSION/${PREVIEW_IMAGE_TAG}/g' myvalues.yaml.template > myvalues.yaml"
                        sh "echo the myvalues.yaml file is:"
                        sh "cat myvalues.yaml"

                        sh "echo creating team: ${TEAM}"

                        sh "git config --global --add user.name JenkinsXBot"
                        sh "git config --global --add user.email jenkins-x@googlegroups.com"

                        sh "cp ./build/linux/jx /usr/bin"

                        sh "jx install --namespace ${TEAM} --helm3 --provider=gke -b --headless --default-admin-password $JENKINS_CREDS_PSW"

                        // lets test we have the jenkins token setup
                        sh "jx get pipeline"
                        
                        sh "echo now running the BDD tests"

                        dir ('/home/jenkins/go/src/github.com/jenkins-x/bdd-jx'){
                            git "https://github.com/jenkins-x/bdd-jx"
                            sh "make configure-ghe"
                            sh "make bootstrap"
                            sh "make test-parallel"
                        }

                        sh "echo now tearing down the team ${TEAM}"
                        sh "jx uninstall -b -y --namespace ${TEAM}"
                    }
                }
            }
        }

        stage('Build and Release') {
            when {
                branch 'master'
            }
            steps {
                dir ('/home/jenkins/go/src/github.com/jenkins-x/jx') {
                    checkout scm
                    container('go') {
                        sh "echo \$(jx-release-version) > pkg/version/VERSION"
                        sh "make release"
                    }
                }
                dir ('/home/jenkins/go/src/github.com/jenkins-x/jx/charts/jx') {
                    container('go') {
                        sh "helm init --client-only"
                        sh "make release"
                    }
                }
                dir ('/home/jenkins/go/src/github.com/jenkins-x/bdd-jx'){
                    sh "git clone https://github.com/jenkins-x/jenkins-x-website.git /home/jenkins/go/src/github.com/jenkins-x/jenkins-x-website"
                    sh "cp -f reports/build-status.md /home/jenkins/go/src/github.com/jenkins-x/jenkins-x-website/content/developing/build-status.md"
                }

                dir("/home/jenkins/go/src/github.com/jenkins-x/jenkins-x-website") {
                  sh 'git config credential.helper store'
                  sh 'git add *'
                  sh "git commit --allow-empty -a -m \"updated build status\""
                  sh "git push origin"
                }
            }
        }
    }
}
