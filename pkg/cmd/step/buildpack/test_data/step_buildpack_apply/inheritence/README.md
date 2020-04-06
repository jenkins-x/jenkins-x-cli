This test case shows how the [pipeline.yaml](pipeline.yaml) layers on the kubernetes workloads capabilities on top of the [base-pipeline.yaml](base-pipeline.yaml) pipeline for classic workloads via inheritance.

By default steps are appended to the base pipeline at each lifecycle section (`setup`, `preBuild`, `build`, `postBuild`, `promote` etc)

In this test we basically append additional steps in [pipeline.yaml](pipeline.yaml) which are appended after the steps defined in [base-pipeline.yaml](base-pipeline.yaml).

To see how to add initial steps or replace steps from the base pipeline see [inheritence2](../inheritence2)

## How the test case works

* the [pipeline.yaml](pipeline.yaml) and [base-pipeline.yaml](base-pipeline.yaml) are used with the [Jenkinsfile.tmpl](Jenkinsfile.tmpl) (which can be reused across languages) to generate a `Jenkinfile`
* the resulting file is asserted to match the expected [Jenkinsfile](Jenkinsfile) 
