package harbor

import (
	"encoding/base64"
	"fmt"
)

type DockerConfig struct {
	Auths map[string]Auth `json:"auths"`
}

type Auth struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Auth     string `json:"auth"`
}

func NewDockerConfig(registry, username, password string) *DockerConfig {
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
	return &DockerConfig{
		Auths: map[string]Auth{
			registry: {Username: username, Password: password, Auth: auth},
		},
	}
}
