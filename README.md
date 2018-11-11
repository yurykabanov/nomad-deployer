# Nomad Deployer

The main purpose of this project is to provide ability to redeploy
[Nomad](https://www.nomadproject.io/) jobs when new tag is pushed to 
[Docker Registry](https://docs.docker.com/registry/).

Currently this project does not provide any control of Nomad jobs, neither new
tasks could be created, nor old tasks could be modified via this manager, it
provides solely redeployment ability at this moment.

## Requirements

1. [Docker Registry](https://docs.docker.com/registry/) as artifact storage
2. [Nomad](https://www.nomadproject.io/) as job scheduler

One also should use some kind of CI capable of building and delivering artifacts
info docker registry, but this project doesn't care about it.

## Scheme of work

Redeployer is an automation tool that reruns existing Nomad jobs when new tag
of docker container is pushed into registry. It listens for events emitted by
registry and then triggers job updates for all corresponding nomad jobs.

This approach requires single convention to be followed: all jobs MUST use
`meta` stanza their definitions to make this tool work.

## Redeployer Configuration

Redeployer utilizes [Viper](https://github.com/spf13/viper) which provides wide
variety of ways to configure application:
- it supports JSON, TOML, YAML, HCL or Java properties formats
- it provides ability to use environment variables, command line flags and
configuration files

Viper will use the following [precedence](https://github.com/spf13/viper#why-viper):
- flags
- environment variables
- configuration file
- defaults

### Command line flags

Read help using:
```bash
./server -h
```

### Environment variables

Environment variables use the following naming convention: any flag with name
`some.flag.value` corresponds to `DEPLOYER_SOME_FLAG_VALUE` variable (note the
prefix `DEPLOYER_`).

### Configuration files

Redeployers will try to search config file in `./`, `./config/` and
`/etc/nomad-deployer/` directories with names `deployer.*` (any format
described above will work).

Example of configuration file in YAML is provided in `./config/` directory.

## Registry configuration

To make described schema work additional configuration of Docker Registry is
required. The config used by registry MUST be overridden:
```bash
docker run -v $PWD/docker_registry_config.yml:/etc/docker/registry/config.yml <...> registry:2
```

Example of such config could be obtained form original container via:
```bash
docker cp registry:/etc/docker/registry/config.yml docker_registry_config.yml
```

The required changes to this config are the following (obviously timeout,
threshold and backoff parameters could be changed as needed):
```yaml
# <...> basic config skipped
notifications:
  endpoints:
    - name: deployer
      url: http://deployer-host:8000/registry/callback
      timeout: 5000ms
      threshold: 5
      backoff: 10s
```

## Nomad configuration

Nomad requires no additional configuration.
 
But all jobs MUST follow given
convention:
- job must have meta stanza with `VERSION` field
- job must use docker image with version interpolation

Example:
```
job "my-awesome-job" {
  meta {
    VERSION = "0.0.1"
  }
  
  group "my-awesome-group" {
    task "my-awesome-task" {
      driver = "docker"
      image = "my-private-registry/something/something:${NOMAD_META_VERSION}"
    }
  }
}
```

## Build from scratch

Without docker:

```bash
go mod download
go build -o ./build/server ./cmd/server/main.go 
```

With docker:
```bash
docker build . -t yurykabanov/nomad-deployer:latest
```
