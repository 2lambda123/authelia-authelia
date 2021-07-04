package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/authelia/authelia/internal/utils"
)

var arch string

var supportedArch = []string{"amd64", "arm32v7", "arm64v8", "coverage"}
var defaultArch = "amd64"
var buildkiteQEMU = os.Getenv("BUILDKITE_AGENT_META_DATA_QEMU")
var ciBranch = os.Getenv("BUILDKITE_BRANCH")
var ciPullRequest = os.Getenv("BUILDKITE_PULL_REQUEST")
var ciTag = os.Getenv("BUILDKITE_TAG")
var dockerTags = regexp.MustCompile(`v(?P<Patch>(?P<Minor>(?P<Major>\d+)\.\d+)\.\d+.*)`)
var ignoredSuffixes = regexp.MustCompile("alpha|beta")
var publicRepo = regexp.MustCompile(`.*:.*`)
var tags = dockerTags.FindStringSubmatch(ciTag)

func init() {
	DockerBuildCmd.PersistentFlags().StringVar(&arch, "arch", defaultArch, "target architecture among: "+strings.Join(supportedArch, ", "))
	DockerPushCmd.PersistentFlags().StringVar(&arch, "arch", defaultArch, "target architecture among: "+strings.Join(supportedArch, ", "))
}

func checkArchIsSupported(arch string) {
	for _, a := range supportedArch {
		if arch == a {
			return
		}
	}

	log.Fatal("Architecture is not supported. Please select one of " + strings.Join(supportedArch, ", ") + ".")
}

func dockerBuildOfficialImage(arch string) error {
	docker := &Docker{}
	// Set default Architecture Dockerfile to amd64.
	dockerfile := "Dockerfile"

	// If not the default value.
	if arch != defaultArch {
		dockerfile = fmt.Sprintf("%s.%s", dockerfile, arch)
	}

	if arch == "arm32v7" || arch == "arm64v8" {
		if buildkiteQEMU != stringTrue {
			err := utils.CommandWithStdout("docker", "run", "--rm", "--privileged", "multiarch/qemu-user-static", "--reset", "-p", "yes").Run()
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	flags, err := getXFlags(ciBranch, os.Getenv("BUILDKITE_BUILD_NUMBER"), "")
	if err != nil {
		log.Fatal(err)
	}

	return docker.Build(IntermediateDockerImageName, dockerfile, ".",
		strings.Join(flags, " "))
}

// DockerBuildCmd Command for building docker image of Authelia.
var DockerBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the docker image of Authelia",
	Run: func(cmd *cobra.Command, args []string) {
		log.Infof("Building Docker image %s...", DockerImageName)
		checkArchIsSupported(arch)
		err := dockerBuildOfficialImage(arch)

		if err != nil {
			log.Fatal(err)
		}

		docker := &Docker{}
		err = docker.Tag(IntermediateDockerImageName, DockerImageName)

		if err != nil {
			log.Fatal(err)
		}
	},
}

// DockerPushCmd Command for pushing Authelia docker image to DockerHub.
var DockerPushCmd = &cobra.Command{
	Use:   "push-image",
	Short: "Publish Authelia docker image to Docker Hub",
	Run: func(cmd *cobra.Command, args []string) {
		log.Infof("Pushing Docker image %s to Docker Hub...", DockerImageName)
		checkArchIsSupported(arch)
		publishDockerImage(arch)
	},
}

// DockerManifestCmd Command for pushing Authelia docker manifest to DockerHub.
var DockerManifestCmd = &cobra.Command{
	Use:   "push-manifest",
	Short: "Publish Authelia docker manifest to Docker Hub",
	Run: func(cmd *cobra.Command, args []string) {
		log.Infof("Pushing Docker manifest of %s to Docker Hub...", DockerImageName)
		publishDockerManifest()
	},
}

func login(docker *Docker, registry string) {
	username := ""
	password := ""

	switch registry {
	case dockerhub:
		username = os.Getenv("DOCKER_USERNAME")
		password = os.Getenv("DOCKER_PASSWORD")
	case ghcr:
		username = os.Getenv("GHCR_USERNAME")
		password = os.Getenv("GHCR_PASSWORD")
	}

	if username == "" {
		log.Fatal(errors.New("DOCKER_USERNAME/GHCR_USERNAME is empty"))
	}

	if password == "" {
		log.Fatal(errors.New("DOCKER_PASSWORD/GHCR_PASSWORD is empty"))
	}

	log.Infof("Login to %s as %s", registry, username)
	err := docker.Login(username, password, registry)

	if err != nil {
		log.Fatalf("Login to %s failed: %s", registry, err)
	}
}

func deploy(docker *Docker, tag, registry string) {
	imageWithTag := registry + "/" + DockerImageName + ":" + tag

	log.Infof("Docker image %s will be deployed on %s", imageWithTag, registry)

	if err := docker.Tag(DockerImageName, imageWithTag); err != nil {
		log.Fatal(err)
	}

	if err := docker.Push(imageWithTag); err != nil {
		log.Fatal(err)
	}
}

func deployManifest(docker *Docker, tag, amd64tag, arm32v7tag, arm64v8tag, registry string) {
	dockerImagePrefix := registry + "/" + DockerImageName + ":"

	log.Infof("Docker manifest %s%s will be deployed on %s", dockerImagePrefix, tag, registry)

	err := docker.Manifest(dockerImagePrefix+tag, dockerImagePrefix+amd64tag, dockerImagePrefix+arm32v7tag, dockerImagePrefix+arm64v8tag)

	if err != nil {
		log.Fatal(err)
	}

	tags := []string{amd64tag, arm32v7tag, arm64v8tag}

	if registry == dockerhub {
		for _, t := range tags {
			log.Infof("Docker removing tag for %s%s on Docker Hub", dockerImagePrefix, t)

			if err := utils.RunFuncWithRetry(3, 10*time.Second, func() (err error) {
				err = docker.CleanTag(t)
				return
			}); err != nil {
				log.Fatal(err)
			}
		}
	}
}

func publishDockerImage(arch string) {
	docker := &Docker{}

	for _, registry := range registries {
		switch {
		case ciTag != "":
			if len(tags) == 4 {
				log.Infof("Detected tags: '%s' | '%s' | '%s'", tags[1], tags[2], tags[3])
				login(docker, registry)
				deploy(docker, tags[1]+"-"+arch, registry)

				if !ignoredSuffixes.MatchString(ciTag) {
					deploy(docker, tags[2]+"-"+arch, registry)
					deploy(docker, tags[3]+"-"+arch, registry)
					deploy(docker, "latest-"+arch, registry)
				}
			} else {
				log.Fatal("Docker image will not be published, the specified tag does not conform to the standard")
			}
		case ciBranch != masterTag && !publicRepo.MatchString(ciBranch):
			login(docker, registry)
			deploy(docker, ciBranch+"-"+arch, registry)
		case ciBranch != masterTag && publicRepo.MatchString(ciBranch):
			login(docker, registry)
			deploy(docker, "PR"+ciPullRequest+"-"+arch, registry)
		case ciBranch == masterTag && ciPullRequest == stringFalse:
			login(docker, registry)
			deploy(docker, "master-"+arch, registry)
		default:
			log.Info("Docker image will not be published")
		}
	}
}

func publishDockerManifest() {
	docker := &Docker{}

	for _, registry := range registries {
		switch {
		case ciTag != "":
			if len(tags) == 4 {
				log.Infof("Detected tags: '%s' | '%s' | '%s'", tags[1], tags[2], tags[3])
				login(docker, registry)
				deployManifest(docker, tags[1], tags[1]+"-amd64", tags[1]+"-arm32v7", tags[1]+"-arm64v8", registry)

				if registry == dockerhub {
					publishDockerReadme(docker)
				}

				if !ignoredSuffixes.MatchString(ciTag) {
					deployManifest(docker, tags[2], tags[2]+"-amd64", tags[2]+"-arm32v7", tags[2]+"-arm64v8", registry)
					deployManifest(docker, tags[3], tags[3]+"-amd64", tags[3]+"-arm32v7", tags[3]+"-arm64v8", registry)
					deployManifest(docker, "latest", "latest-amd64", "latest-arm32v7", "latest-arm64v8", registry)

					if registry == dockerhub {
						publishDockerReadme(docker)
						updateMicroBadger(docker)
					}
				}
			} else {
				log.Fatal("Docker manifest will not be published, the specified tag does not conform to the standard")
			}
		case ciBranch != masterTag && !publicRepo.MatchString(ciBranch):
			login(docker, registry)
			deployManifest(docker, ciBranch, ciBranch+"-amd64", ciBranch+"-arm32v7", ciBranch+"-arm64v8", registry)
		case ciBranch != masterTag && publicRepo.MatchString(ciBranch):
			login(docker, registry)
			deployManifest(docker, "PR"+ciPullRequest, "PR"+ciPullRequest+"-amd64", "PR"+ciPullRequest+"-arm32v7", "PR"+ciPullRequest+"-arm64v8", registry)
		case ciBranch == masterTag && ciPullRequest == stringFalse:
			login(docker, registry)
			deployManifest(docker, "master", "master-amd64", "master-arm32v7", "master-arm64v8", registry)

			if registry == dockerhub {
				publishDockerReadme(docker)
			}
		default:
			log.Info("Docker manifest will not be published")
		}
	}
}

func publishDockerReadme(docker *Docker) {
	log.Info("Docker pushing README.md to Docker Hub")

	if err := docker.PublishReadme(); err != nil {
		log.Fatal(err)
	}
}
func updateMicroBadger(docker *Docker) {
	log.Info("Updating MicroBadger metadata from Docker Hub")

	if err := docker.UpdateMicroBadger(); err != nil {
		log.Fatal(err)
	}
}
