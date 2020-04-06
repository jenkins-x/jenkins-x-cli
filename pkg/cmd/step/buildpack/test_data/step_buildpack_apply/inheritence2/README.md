This test case shows how the [pipeline.yaml](pipeline.yaml) can replace or add init steps on top of the [base-pipeline.yaml](base-pipeline.yaml) pipeline via inheritance.

* to add steps before a base pipeline's lifecycle use `preSteps` like [this example](pipeline.yaml#L6-L9)
* to add steps after a base pipelines lifecycle use `steps` like [this example](pipeline.yaml#L10-L13)
* to replace the steps from the base lifecycle use `steps` and specify `replace: true` like [this example](pipeline.yaml#L15-L19)

## How the test case works

* the [pipeline.yaml](pipeline.yaml) and [base-pipeline.yaml](base-pipeline.yaml) are used with the [Jenkinsfile.tmpl](Jenkinsfile.tmpl) (which can be reused across languages) to generate a `Jenkinfile`
* the resulting file is asserted to match the expected [Jenkinsfile](Jenkinsfile) 
