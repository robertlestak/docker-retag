package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

var (
	Version          string = "dev"
	Username         string
	Password         string
	dockerRetagFlags = flag.NewFlagSet("docker-retag", flag.ExitOnError)
)

type Manifest struct {
	MediaType     string `json:"mediaType"`
	SchemaVersion int    `json:"schemaVersion"`
	Config        struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		Size      int    `json:"size"`
	} `json:"config"`
	Layers []struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		Size      int    `json:"size"`
	} `json:"layers"`
}

func registryProtocol(registry string) string {
	l := log.WithFields(log.Fields{
		"package":  "main",
		"fn":       "registryProtocol",
		"registry": registry,
	})
	l.Debug("Getting registry protocol")
	if os.Getenv("INSECURE_REGISTRY") == "true" {
		return "http"
	}
	return "https"
}

func registryAuth(registry string) (string, error) {
	l := log.WithFields(log.Fields{
		"package":  "main",
		"registry": registry,
		"fn":       "registryAuth",
	})
	l.Debug("Getting registry auth")
	// get auth from keychain
	// if no auth is found, return empty string
	// if auth is found, return base64 encoded string
	if Username != "" && Password != "" {
		l.Debug("Using username and password")
		return base64.StdEncoding.EncodeToString([]byte(Username + ":" + Password)), nil
	}
	if os.Getenv("DOCKER_USER") != "" && os.Getenv("DOCKER_PASS") != "" {
		l.Debug("Using docker credentials")
		return base64.StdEncoding.EncodeToString([]byte(os.Getenv("DOCKER_USER") + ":" + os.Getenv("DOCKER_PASS"))), nil
	}
	// check docker config
	l.Debug("Checking docker config")
	dockerConfig := os.Getenv("HOME") + "/.docker/config.json"
	if _, err := os.Stat(dockerConfig); err == nil {
		l.Debug("Docker config found")
		// docker config found
		// read docker config
		bd, err := ioutil.ReadFile(dockerConfig)
		if err != nil {
			l.Error("Error reading docker config: ", err)
			return "", err
		}
		// parse docker config
		var dc map[string]interface{}
		err = json.Unmarshal(bd, &dc)
		if err != nil {
			l.Error("Error parsing docker config: ", err)
			return "", err
		}
		// get auths
		auths := dc["auths"].(map[string]interface{})
		// get auth for registry
		auth := auths[registry].(map[string]interface{})
		// get auth string
		if auth == nil || auth["auth"] == nil {
			l.Debug("No auth found for registry")
			return "", nil
		}
		authString := auth["auth"].(string)
		return authString, nil
	}
	l.Debug("Docker config not found")
	return "", nil
}

func urlToImageTag(url string) (string, string, string, error) {
	l := log.WithFields(log.Fields{
		"package": "main",
		"fn":      "urlToImageTag",
		"url":     url,
	})
	l.Debug("Getting image and tag from url")
	// split the url into the registry, image, and tag
	// url in format: hello.example.com:5000/myimage/path:latest
	// it can also be in format: myimage/path:latest
	// if no registry is specified, it will use the default registry (docker.io)
	// if no tag is specified, it will use the default tag (latest)
	// registry: hello.example.com:5000
	// image: myimage/path
	// tag: latest
	var registry, image, tag string
	if strings.Contains(url, "/") {
		// url has a registry
		// split the url into registry and image
		splitUrl := strings.Split(url, "/")
		registry = splitUrl[0]
		image = strings.Join(splitUrl[1:], "/")
	} else {
		// url does not have a registry
		// use the default registry
		registry = "index.docker.io"
		image = url
	}
	if strings.Contains(image, ":") {
		// image has a tag
		// split the image into image and tag
		splitImage := strings.Split(image, ":")
		image = splitImage[0]
		tag = splitImage[1]
	} else {
		// image does not have a tag
		// use the default tag
		tag = "latest"
	}
	l = l.WithFields(log.Fields{
		"registry": registry,
		"image":    image,
		"tag":      tag,
	})
	l.Debug("Got image and tag from url")
	return registry, image, tag, nil
}

func getManifest(url string) (Manifest, error) {
	l := log.WithFields(log.Fields{
		"package": "main",
		"func":    "getManifest",
		"url":     url,
	})
	l.Debug("Getting manifest from ", url)
	var m Manifest
	registry, image, tag, err := urlToImageTag(url)
	if err != nil {
		l.Error("Error getting image and tag from url: ", err)
		return m, err
	}
	protocol := registryProtocol(registry)
	l.Debug("Registry: ", registry)
	l.Debug("Image: ", image)
	l.Debug("Tag: ", tag)
	auth, err := registryAuth(registry)
	if err != nil {
		l.Error("Error getting registry auth: ", err)
		return m, err
	}
	manifestUrl := fmt.Sprintf("%s://%s/v2/%s/manifests/%s", protocol, registry, image, tag)
	l = l.WithFields(log.Fields{
		"manifestUrl": manifestUrl,
	})
	l.Debug("Manifest url: ", manifestUrl)
	c := &http.Client{}
	req, err := http.NewRequest("GET", manifestUrl, nil)
	if err != nil {
		l.Error("Error creating request: ", err)
		return m, err
	}
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	if auth != "" {
		req.Header.Add("Authorization", "Basic "+auth)
	}
	resp, err := c.Do(req)
	if err != nil {
		l.Error("Error getting manifest: ", err)
		return m, err
	}
	defer resp.Body.Close()
	bd, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		l.Error("Error reading response body: ", err)
		return m, err
	}
	if resp.StatusCode != 200 {
		l.Error("Error getting manifest: ", resp.Status)
		return m, errors.New(resp.Status)
	}
	l.Debug("Manifest: ", string(bd))
	err = json.Unmarshal(bd, &m)
	if err != nil {
		l.Error("Error unmarshalling manifest: ", err)
		return m, err
	}
	return m, nil
}

func uploadManifest(url string, manifest Manifest) error {
	l := log.WithFields(log.Fields{
		"package": "main",
		"func":    "uploadManifest",
		"url":     url,
	})
	l.Debug("Uploading manifest to ", url)
	registry, image, tag, err := urlToImageTag(url)
	if err != nil {
		l.Error("Error getting image and tag from url: ", err)
		return err
	}
	protocol := registryProtocol(registry)
	l.Debug("Registry: ", registry)
	l.Debug("Image: ", image)
	l.Debug("Tag: ", tag)
	auth, err := registryAuth(registry)
	if err != nil {
		l.Error("Error getting registry auth: ", err)
		return err
	}
	manifestUrl := fmt.Sprintf("%s://%s/v2/%s/manifests/%s", protocol, registry, image, tag)
	l = l.WithFields(log.Fields{
		"manifestUrl": manifestUrl,
	})
	l.Debug("Manifest url: ", manifestUrl)
	l.Debug("Manifest: ", manifest)
	c := &http.Client{}
	jd, err := json.Marshal(manifest)
	if err != nil {
		l.Error("Error marshalling manifest: ", err)
		return err
	}
	data := bytes.NewBuffer(jd)
	req, err := http.NewRequest("PUT", manifestUrl, data)
	if err != nil {
		l.Error("Error creating request: ", err)
		return err
	}
	req.Header.Add("Content-Type", manifest.MediaType)
	if auth != "" {
		req.Header.Add("Authorization", "Basic "+auth)
	}
	resp, err := c.Do(req)
	if err != nil {
		l.Error("Error getting manifest: ", err)
		return err
	}
	defer resp.Body.Close()
	bd, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		l.Error("Error reading response body: ", err)
		return err
	}
	l.Debug("Response: ", string(bd))
	if resp.StatusCode != 201 {
		l.Error("Error uploading manifest: ", resp.Status)
		return errors.New(resp.Status)
	}
	return nil
}

func init() {
	ll, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		ll = log.InfoLevel
	}
	log.SetLevel(ll)
}

type UploadJob struct {
	Manifest Manifest
	Image    string
}

func manifestUploadWorker(jobs <-chan UploadJob, results chan<- error) {
	for j := range jobs {
		err := uploadManifest(j.Image, j.Manifest)
		results <- err
	}
}

func usage() {
	fmt.Println("Usage: docker-retag [flags] <image> <new tag> ...")
	fmt.Println("Flags:")
	dockerRetagFlags.PrintDefaults()
}

func version() {
	fmt.Println("docker-retag version: ", Version)
}

func main() {
	l := log.WithFields(log.Fields{
		"package": "main",
		"func":    "main",
	})
	l.Debug("Starting docker-retag")
	dockerRetagFlags.Usage = usage
	username := dockerRetagFlags.String("u", "", "Username for registry")
	password := dockerRetagFlags.String("p", "", "Password for registry")
	passwordStdin := dockerRetagFlags.Bool("P", false, "Read password from stdin")
	versionFlag := dockerRetagFlags.Bool("v", false, "Print version and exit")
	dockerRetagFlags.Parse(os.Args[1:])
	args := dockerRetagFlags.Args()
	l.Debug("Args: ", args)
	// usage of the function
	// "docker-retag [flags] <image> <new tag> ..."
	if *versionFlag {
		version()
		os.Exit(0)
	}
	if len(args) < 2 {
		usage()
		os.Exit(1)
	}
	Username = *username
	Password = *password
	if *passwordStdin {
		// read password from stdin
		bd, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			l.Error("Error reading password from stdin: ", err)
		}
		Password = strings.TrimSpace(string(bd))
	}
	l.Debug("Username: ", Username)
	l.Debug("Password: ", Password)
	image := args[0]
	newImages := args[1:]
	l = l.WithFields(log.Fields{
		"image":      image,
		"new_images": newImages,
	})
	l.Debug("Retagging image")
	// get original manifest
	manifest, err := getManifest(image)
	if err != nil {
		l.Error("Error getting manifest: ", err)
		os.Exit(1)
	}
	l.Debug("Got manifest")
	// upload manifest to new images
	workers := 10
	if len(newImages) < workers {
		workers = len(newImages)
	}
	jobs := make(chan UploadJob, len(newImages))
	results := make(chan error, len(newImages))
	for i := 0; i < workers; i++ {
		go manifestUploadWorker(jobs, results)
	}
	for _, newImage := range newImages {
		jobs <- UploadJob{
			Manifest: manifest,
			Image:    newImage,
		}
	}
	close(jobs)
	for i := 0; i < len(newImages); i++ {
		err := <-results
		if err != nil {
			l.Error("Error uploading manifest: ", err)
			os.Exit(1)
		}
	}
}
