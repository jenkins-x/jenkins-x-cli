This test case shows how the [pipeline.yaml](pipeline.yaml) layers on the kubernetes workloads capabilities on top of the imported build pack at [import_dir/classic/maven/pipeline.yaml](import_dir/classic/maven/pipeline.yaml) pipeline for classic workloads via inheritance.

This test imports a base pipeline file from a module via the [extends import directive](pipeline.yaml#L1-L3).

We can then resolve the named module using different strategies via `jenkinsfile.ImportFileResolver` in the `jx step create jenkinsfile` command.


## How the test case works

* the [pipeline.yaml](pipeline.yaml) inherits its base pipeline [import_dir/classic/maven/pipeline.yaml](import_dir/classic/maven/pipeline.yaml) via the [extends import directive](pipeline.yaml#L1-L3)
* the resulting pipeline is combined with the [Jenkinsfile.tmpl](Jenkinsfile.tmpl) (which can be reused across languages) to generate a `Jenkinfile`
* the resulting file is asserted to match the expected [Jenkinsfile](Jenkinsfile) 
