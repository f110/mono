package config

import (
	"io/ioutil"
	"os"

	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

type Config struct {
	WebhookListener         string      `json:"webhook_listener"`
	BuildNamespace          string      `json:"build_namespace"`
	GitHubTokenFile         string      `json:"github_token_file"`
	GitHubAppId             int64       `json:"app_id"`
	GitHubInstallationId    int64       `json:"installation_id"`
	GitHubAppPrivateKeyFile string      `json:"app_private_key_file"`
	PrivateKeySecretName    string      `json:"private_key_secret_name"`
	StorageHost             string      `json:"storage_host"`
	StorageTokenSecretName  string      `json:"storage_token_secret_name"`
	ArtifactBucket          string      `json:"artifact_bucket"`
	HostAliases             []HostAlias `json:"host_aliases"`
	CommitAuthor            string      `json:"commit_author"`
	CommitEmail             string      `json:"commit_email"`
	AllowRepositories       []string    `json:"allow_repositories"`
	SafeMode                bool        `json:"safe_mode"`

	GitHubToken string `json:"-"`
}

type HostAlias struct {
	Hostnames []string `json:"hostnames"`
	IP        string   `json:"ip"`
}

func ReadConfig(p string) (*Config, error) {
	b, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, xerrors.Errorf(": %v", err)
	}

	conf := &Config{}
	if err := yaml.Unmarshal(b, conf); err != nil {
		return nil, xerrors.Errorf(": %v", err)
	}
	if conf.BuildNamespace == "" {
		conf.BuildNamespace = os.Getenv("POD_NAMESPACE")
	}
	if conf.BuildNamespace == "" {
		return nil, xerrors.New("config: build namespace is mandatory")
	}
	if conf.GitHubTokenFile != "" {
		b, err := ioutil.ReadFile(conf.GitHubTokenFile)
		if err != nil {
			return nil, xerrors.Errorf(": %v", err)
		}
		conf.GitHubToken = string(b)
	}

	return conf, nil
}

type BuildRule struct {
	Branch                 string       `json:"branch"`
	Private                bool         `json:"private"`
	BazelVersion           string       `json:"bazel_version"`
	Target                 string       `json:"target"`
	DockerConfigSecretName string       `json:"docker_config_secret_name"`
	Artifacts              []string     `json:"artifacts"`
	Env                    []Env        `json:"env"`
	PostProcess            *PostProcess `json:"post_process"`
}

type PostProcess struct {
	Repo  string   `json:"repo"`
	Image string   `json:"image"`
	Paths []string `json:"paths"`
}

type Env struct {
	Name   string        `json:"name"`
	Value  string        `json:"value,omitempty"`
	Secret *SecretSource `json:"secret,omitempty"`
}

type SecretSource struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

func ParseBuildRule(v string) (*BuildRule, error) {
	conf := &BuildRule{}
	if err := yaml.Unmarshal([]byte(v), conf); err != nil {
		return nil, xerrors.Errorf(": %v", err)
	}

	return conf, nil
}

func (e Env) ToEnvVar() corev1.EnvVar {
	v := corev1.EnvVar{
		Name:  e.Name,
		Value: e.Value,
	}
	if e.Secret != nil {
		v.ValueFrom = &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: e.Secret.Name,
				},
				Key: e.Secret.Key,
			},
		}
	}

	return v
}

type DNSControlRule struct {
	MasterBranch string          `json:"master_branch"`
	Image        string          `json:"image"`
	Dir          string          `json:"dir"`
	Secret       *SecretSelector `json:"secret"`
}

type SecretSelector struct {
	EnvName string `json:"env_name"`
	Name    string `json:"name"`
	Key     string `json:"key"`
}

func ParseDNSControlRule(v string) (*DNSControlRule, error) {
	conf := &DNSControlRule{}
	if err := yaml.Unmarshal([]byte(v), conf); err != nil {
		return nil, xerrors.Errorf(": %v", err)
	}

	return conf, nil
}
