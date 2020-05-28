package storage

import (
	"fmt"
	msg "hash_interface/internal/storage/message"
	"hash_interface/tools"
	"log"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	ctx "golang.org/x/net/context"
)

type DockerWrapper struct {
	Context *ctx.Context
	Client  *client.Client
}

var docker DockerWrapper

var restartTimeOutDuration = 5 * time.Second

func init() {
	if err := docker.checkInit(); err != nil {
		tools.ErrorLogger.Println(msg.DockerInitFail)
		log.Fatal(err)
	}
}

func (docker DockerWrapper) restartRedisContainer(targetIP string) error {

	redisContainer, err := docker.getContainerWithIP(targetIP)
	if err != nil {
		return err
	}

	// tools.InfoLogger.Printf(
	// 	msg.ContainerStatus,
	// 	targetIP,
	// 	redisContainer.Status,
	// )

	switch redisContainer.Status {

	case "restarting", "running":
		break

	default:
		tools.InfoLogger.Printf(
			msg.ContainerRestart,
			targetIP,
		)

		err = docker.Client.ContainerRestart(
			(*docker.Context),
			redisContainer.ID,
			&restartTimeOutDuration,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func dockerClientSetUp() error {

	var err error

	context := ctx.Background()
	docker.Context = &context

	// Use Env For docker sdk client setup
	docker.Client, err = client.NewEnvClient()
	if err != nil {
		return err
	}

	return nil
}

func (docker DockerWrapper) checkInit() error {
	if docker.Client == nil || docker.Context == nil {
		if err := dockerClientSetUp(); err != nil {
			return err
		}
	}

	return nil
}

func (docker DockerWrapper) getContainerWithIP(targetIP string) (types.Container, error) {

	var err error

	if err := docker.checkInit(); err != nil {
		tools.ErrorLogger.Println(msg.DockerInitFail)
		return types.Container{}, err
	}

	containerListOption := types.ContainerListOptions{
		All: true,
	}

	containers, err := docker.Client.ContainerList(
		*docker.Context,
		containerListOption,
	)

	if err != nil {
		return types.Container{}, err
	}

	// tools.InfoLogger.Println(msg.TargetContainerIP, targetIP)

	for _, eachContainer := range containers {
		for _, endPointSetting := range eachContainer.NetworkSettings.Networks {
			if strings.Contains(targetIP, endPointSetting.IPAddress) {

				// tools.InfoLogger.Println(
				// 	msg.ContainerFound,
				// 	eachContainer.Names[0],
				// 	endPointSetting.IPAddress,
				// )

				return eachContainer, nil
			}
		}
	}

	return types.Container{}, fmt.Errorf(msg.ContainerNotFound, targetIP)
}
