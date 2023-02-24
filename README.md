# docker-retag

A simple tool to retag manifests in a remote manifest without having to pull the image locally.

This is only compatible with [v2 manifest format](https://docs.docker.com/registry/spec/manifest-v2-2/).

## Usage

```bash
Usage: docker-retag [flags] <image> <new tag> ...
Flags:
  -P    Read password from stdin
  -p string
        Password for registry
  -u string
        Username for registry
  -v    Print version and exit
```

## Example

```bash
docker-retag registry.example.com/hello-world:v0.0.1 registry.example.com/hello-world:main registry.example.com/hello-world:latest
```

### With Auth

```bash
docker-retag -u username -p password registry.example.com/hello-world:v0.0.1 registry.example.com/hello-world:main
# or
echo password | docker-retag -u username -P registry.example.com/hello-world:v0.0.1 registry.example.com/hello-world:main
# or
export DOCKER_USER=username
export DOCKER_PASS=password
docker-retag registry.example.com/hello-world:v0.0.1 registry.example.com/hello-world:main
# finally, it will fall back to checking ~/.docker/config.json for any inline auths for the registry
```

## Run in Docker

```bash
docker run --rm -it -v ~/.docker/config.json:/root/.docker/config.json:ro \
	docker-retag registry.example.com/hello-world:v0.0.1 registry.example.com/hello-world:main
```