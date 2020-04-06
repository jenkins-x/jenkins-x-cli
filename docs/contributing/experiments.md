# Experiments and feature flags

This page contains a list of experiments that are being worked on and how to enable them.  This list will be maintained and
it will likely change how features are enabled as they mature past an experiment. 

Experiments tend to go hand in hand with the Jenkins X enhancement process of which more details can be found in the git
repository [https://github.com/jenkins-x/enhancements](https://github.com/jenkins-x/enhancements)

# Current Experiments

## Helmfile

https://github.com/jenkins-x/enhancements/pull/1

We are experimenting with [Helmfile](https://github.com/roboll/helmfile) to see if we can make the `jx boot` implementation
a bit more modular and leverage some of the extra fetures the OSS project has.  To enable this feature set a top level
jx requirements value:

```yaml
helmfile: true
```

