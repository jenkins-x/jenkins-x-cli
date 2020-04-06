package jenkins

import (
	"github.com/jenkins-x/jx/pkg/gits"
	"k8s.io/apimachinery/pkg/util/uuid"
)

// CreateFolderXML creates a Jenkins Folder XML
func CreateFolderXML(folderURL string, name string) string {
	return `<?xml version='1.0' encoding='UTF-8'?>
<com.cloudbees.hudson.plugins.folder.Folder plugin="cloudbees-folder@6.2.1">
  <actions>
    <io.jenkins.blueocean.service.embedded.BlueOceanUrlAction plugin="blueocean-rest-impl@1.3.3">
      <blueOceanUrlObject class="io.jenkins.blueocean.service.embedded.BlueOceanUrlObjectImpl">
        <mappedUrl>blue/organizations/jenkins</mappedUrl>
        <modelObject class="com.cloudbees.hudson.plugins.folder.Folder" reference="../../../.."/>
      </blueOceanUrlObject>
    </io.jenkins.blueocean.service.embedded.BlueOceanUrlAction>
  </actions>
  <description></description>
  <properties>
    <org.jenkinsci.plugins.pipeline.modeldefinition.config.FolderConfig plugin="pipeline-model-definition@1.2.4">
      <dockerLabel></dockerLabel>
      <registry plugin="docker-commons@1.9"/>
    </org.jenkinsci.plugins.pipeline.modeldefinition.config.FolderConfig>
  </properties>
  <folderViews class="com.cloudbees.hudson.plugins.folder.views.DefaultFolderViewHolder">
    <views>
      <hudson.model.AllView>
        <owner class="com.cloudbees.hudson.plugins.folder.Folder" reference="../../../.."/>
        <name>All</name>
        <filterExecutors>false</filterExecutors>
        <filterQueue>false</filterQueue>
        <properties class="hudson.model.View$PropertyList"/>
      </hudson.model.AllView>
    </views>
    <tabBar class="hudson.views.DefaultViewsTabBar"/>
  </folderViews>
  <healthMetrics>
    <com.cloudbees.hudson.plugins.folder.health.WorstChildHealthMetric>
      <nonRecursive>false</nonRecursive>
    </com.cloudbees.hudson.plugins.folder.health.WorstChildHealthMetric>
  </healthMetrics>
  <icon class="com.cloudbees.hudson.plugins.folder.icons.StockFolderIcon"/>
</com.cloudbees.hudson.plugins.folder.Folder>
`
}

// CreatePipelineXML creates the XML for a stand alone pipeline that is not using the Multi Branch Project
func CreatePipelineXML(gitURL string, branch string, jenksinsfileName string) string {
	return `<?xml version='1.1' encoding='UTF-8'?>
<flow-definition plugin="workflow-job@2.31">
  <actions>
    <org.jenkinsci.plugins.pipeline.modeldefinition.actions.DeclarativeJobAction plugin="pipeline-model-definition@1.3.4.1"/>
  </actions>
  <description></description>
  <keepDependencies>false</keepDependencies>
  <properties/>
  <definition class="org.jenkinsci.plugins.workflow.cps.CpsScmFlowDefinition" plugin="workflow-cps@2.63">
    <scm class="hudson.plugins.git.GitSCM" plugin="git@3.9.1">
      <configVersion>2</configVersion>
      <userRemoteConfigs>
        <hudson.plugins.git.UserRemoteConfig>
          <url>` + gitURL + `</url>
        </hudson.plugins.git.UserRemoteConfig>
      </userRemoteConfigs>
      <branches>
        <hudson.plugins.git.BranchSpec>
          <name>*/` + branch + `</name>
        </hudson.plugins.git.BranchSpec>
      </branches>
      <doGenerateSubmoduleConfigurations>false</doGenerateSubmoduleConfigurations>
      <submoduleCfg class="list"/>
      <extensions/>
    </scm>
    <scriptPath>` + jenksinsfileName + `</scriptPath>
    <lightweight>true</lightweight>
  </definition>
  <triggers/>
  <disabled>false</disabled>
</flow-definition>`
}

func createBranchSource(info *gits.GitRepository, gitProvider gits.GitProvider, credentials string, branches string) string {
	idXml := `<id>` + string(uuid.NewUUID()) + `</id>`
	credXml := ""
	if credentials != "" {
		credXml = `
		  <credentialsId>` + credentials + `</credentialsId>
`
	}

	switch gitProvider.Kind() {
	case gits.KindGitHub:
		serverXml := ""
		ghp, ok := gitProvider.(*gits.GitHubProvider)
		if ok {
			u := ghp.GetEnterpriseApiURL()
			if u != "" {
				serverXml = `		  <apiUri>` + u + `</apiUri>
`
			}
		}
		return `
	    <source class="org.jenkinsci.plugins.github_branch_source.GitHubSCMSource" plugin="github-branch-source@2.3.1">
		  ` + idXml + credXml + serverXml + `
		  <repoOwner>` + info.Organisation + `</repoOwner>
		  <repository>` + info.Name + `</repository>
		  <traits>
			<org.jenkinsci.plugins.github__branch__source.BranchDiscoveryTrait>
			  <strategyId>1</strategyId>
			</org.jenkinsci.plugins.github__branch__source.BranchDiscoveryTrait>
			<org.jenkinsci.plugins.github__branch__source.OriginPullRequestDiscoveryTrait>
			  <strategyId>2</strategyId>
			</org.jenkinsci.plugins.github__branch__source.OriginPullRequestDiscoveryTrait>
			<org.jenkinsci.plugins.github__branch__source.ForkPullRequestDiscoveryTrait>
			  <strategyId>1</strategyId>
			  <trust class="org.jenkinsci.plugins.github_branch_source.ForkPullRequestDiscoveryTrait$TrustContributors"/>
			</org.jenkinsci.plugins.github__branch__source.ForkPullRequestDiscoveryTrait>
			<jenkins.scm.impl.trait.RegexSCMHeadFilterTrait plugin="scm-api@2.2.6">
			  <regex>` + branches + `</regex>
			</jenkins.scm.impl.trait.RegexSCMHeadFilterTrait>
		  </traits>
		</source>
`
	case gits.KindGitea:
		return `
	    <source class="org.jenkinsci.plugin.gitea.GiteaSCMSource" plugin="gitea@1.0.5">
          ` + idXml + credXml + `
          <serverUrl>` + info.HostURLWithoutUser() + `</serverUrl>
          <repoOwner>` + info.Organisation + `</repoOwner>
		  <repository>` + info.Name + `</repository>
          <traits>
            <org.jenkinsci.plugin.gitea.BranchDiscoveryTrait>
              <strategyId>1</strategyId>
            </org.jenkinsci.plugin.gitea.BranchDiscoveryTrait>
            <org.jenkinsci.plugin.gitea.OriginPullRequestDiscoveryTrait>
              <strategyId>2</strategyId>
            </org.jenkinsci.plugin.gitea.OriginPullRequestDiscoveryTrait>
            <org.jenkinsci.plugin.gitea.ForkPullRequestDiscoveryTrait>
              <strategyId>1</strategyId>
              <trust class="org.jenkinsci.plugin.gitea.ForkPullRequestDiscoveryTrait$TrustContributors"/>
            </org.jenkinsci.plugin.gitea.ForkPullRequestDiscoveryTrait>
			<jenkins.scm.impl.trait.RegexSCMHeadFilterTrait plugin="scm-api@2.2.6">
			  <regex>` + branches + `</regex>
			</jenkins.scm.impl.trait.RegexSCMHeadFilterTrait>
		  </traits>
		</source>
`
	case gits.KindBitBucketCloud, gits.KindBitBucketServer:
		return `
	 	<source class="com.cloudbees.jenkins.plugins.bitbucket.BitbucketSCMSource" plugin="cloudbees-bitbucket-branch-source@2.2.10">
	 	  ` + idXml + credXml + `
          <serverUrl>` + info.HostURLWithoutUser() + `</serverUrl>
          <repoOwner>` + info.Organisation + `</repoOwner>
		  <repository>` + info.Name + `</repository>
	 	  <traits>
	 	    <com.cloudbees.jenkins.plugins.bitbucket.BranchDiscoveryTrait>
	 	      <strategyId>1</strategyId>
	 	    </com.cloudbees.jenkins.plugins.bitbucket.BranchDiscoveryTrait>
	 	    <com.cloudbees.jenkins.plugins.bitbucket.OriginPullRequestDiscoveryTrait>
	 	      <strategyId>2</strategyId>
	 	    </com.cloudbees.jenkins.plugins.bitbucket.OriginPullRequestDiscoveryTrait>
	 	    <com.cloudbees.jenkins.plugins.bitbucket.ForkPullRequestDiscoveryTrait>
	 	      <strategyId>1</strategyId>
	 	      <trust class="com.cloudbees.jenkins.plugins.bitbucket.ForkPullRequestDiscoveryTrait$TrustTeamForks"/>
	 	    </com.cloudbees.jenkins.plugins.bitbucket.ForkPullRequestDiscoveryTrait>
			<jenkins.scm.impl.trait.RegexSCMHeadFilterTrait plugin="scm-api@2.2.6">
			  <regex>` + branches + `</regex>
			</jenkins.scm.impl.trait.RegexSCMHeadFilterTrait>
	 	  </traits>
	 	</source>
`
	}

	return `
<source class="jenkins.plugins.git.GitSCMSource" plugin="git@3.7.0">
  ` + idXml + `
  <remote>` + info.URL + `</remote>
` + credXml + `
<traits>
	<jenkins.plugins.git.traits.BranchDiscoveryTrait/>
  </traits>
</source>
<strategy class="jenkins.branch.DefaultBranchPropertyStrategy">
  <properties class="empty-list"/>
</strategy>
`
}

func CreateMultiBranchProjectXml(info *gits.GitRepository, gitProvider gits.GitProvider, credentials string, branches string, jenkinsfile string) string {
	triggerXml := `
	  <triggers>
	    <com.cloudbees.hudson.plugins.folder.computed.PeriodicFolderTrigger plugin="cloudbees-folder@6.3">
	      <spec>H/12 * * * *</spec>
	      <interval>300000</interval>
	    </com.cloudbees.hudson.plugins.folder.computed.PeriodicFolderTrigger>
	  </triggers>
`
	return `<?xml version='1.0' encoding='UTF-8'?>
<org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject plugin="workflow-multibranch@2.16">
  <actions/>
  <description></description>
  <properties>
	<org.jenkinsci.plugins.pipeline.modeldefinition.config.FolderConfig plugin="pipeline-model-definition@1.2.4">
	  <dockerLabel></dockerLabel>
	  <registry plugin="docker-commons@1.9"/>
	</org.jenkinsci.plugins.pipeline.modeldefinition.config.FolderConfig>
  </properties>
  <folderViews class="jenkins.branch.MultiBranchProjectViewHolder" plugin="branch-api@2.0.15">
	<owner class="org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject" reference="../.."/>
  </folderViews>
  <healthMetrics>
	<com.cloudbees.hudson.plugins.folder.health.WorstChildHealthMetric plugin="cloudbees-folder@6.2.1">
	  <nonRecursive>false</nonRecursive>
	</com.cloudbees.hudson.plugins.folder.health.WorstChildHealthMetric>
  </healthMetrics>
  <icon class="jenkins.branch.MetadataActionFolderIcon" plugin="branch-api@2.0.15">
	<owner class="org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject" reference="../.."/>
  </icon>
  <orphanedItemStrategy class="com.cloudbees.hudson.plugins.folder.computed.DefaultOrphanedItemStrategy" plugin="cloudbees-folder@6.2.1">
	<pruneDeadBranches>true</pruneDeadBranches>
	<daysToKeep>-1</daysToKeep>
	<numToKeep>-1</numToKeep>
  </orphanedItemStrategy>
` + triggerXml + `
  <disabled>false</disabled>
  <sources class="jenkins.branch.MultiBranchProject$BranchSourceList" plugin="branch-api@2.0.15">
	<data>
	  <jenkins.branch.BranchSource>
` + createBranchSource(info, gitProvider, credentials, branches) + `
		<strategy class="jenkins.branch.DefaultBranchPropertyStrategy">
		  <properties class="empty-list"/>
		</strategy>
	  </jenkins.branch.BranchSource>
	</data>
	<owner class="org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject" reference="../.."/>
  </sources>
  <factory class="org.jenkinsci.plugins.workflow.multibranch.WorkflowBranchProjectFactory">
	<owner class="org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject" reference="../.."/>
	<scriptPath>` + jenkinsfile + `</scriptPath>
  </factory>
</org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject>
`
}
