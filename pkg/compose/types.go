package compose

import (
	"time"
)

type ComposeFile struct {
	Version  string                 `yaml:"version"`
	Services map[string]*Service    `yaml:"services"`
	Networks map[string]*Network    `yaml:"networks,omitempty"`
	Volumes  map[string]*Volume     `yaml:"volumes,omitempty"`
	Configs  map[string]*Config     `yaml:"configs,omitempty"`
	Secrets  map[string]*Secret     `yaml:"secrets,omitempty"`
	Extensions map[string]interface{} `yaml:"x-,inline"`
}

type Service struct {
	Image           string                 `yaml:"image,omitempty"`
	Build           *BuildConfig          `yaml:"build,omitempty"`
	Command         []string              `yaml:"command,omitempty"`
	Entrypoint      []string              `yaml:"entrypoint,omitempty"`
	Environment     map[string]string     `yaml:"environment,omitempty"`
	EnvFile         []string              `yaml:"env_file,omitempty"`
	Ports           []string              `yaml:"ports,omitempty"`
	Volumes         []string              `yaml:"volumes,omitempty"`
	Networks        []string              `yaml:"networks,omitempty"`
	DependsOn       map[string]DependsOn  `yaml:"depends_on,omitempty"`
	Deploy          *DeployConfig         `yaml:"deploy,omitempty"`
	HealthCheck     *HealthCheck          `yaml:"healthcheck,omitempty"`
	Labels          map[string]string     `yaml:"labels,omitempty"`
	Restart         string                `yaml:"restart,omitempty"`
	InitContainers  []InitContainer       `yaml:"init_containers,omitempty"`
	PostContainers  []PostContainer       `yaml:"post_containers,omitempty"`
	Hooks           *Hooks                `yaml:"hooks,omitempty"`
	CloudNative     *CloudNativeConfig    `yaml:"cloud_native,omitempty"`
}

type InitContainer struct {
	Name        string            `yaml:"name"`
	Image       string            `yaml:"image"`
	Command     []string          `yaml:"command,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty"`
	Volumes     []string          `yaml:"volumes,omitempty"`
	Resources   *Resources        `yaml:"resources,omitempty"`
}

type PostContainer struct {
	Name        string            `yaml:"name"`
	Image       string            `yaml:"image"`
	Command     []string          `yaml:"command,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty"`
	Volumes     []string          `yaml:"volumes,omitempty"`
	WaitFor     string            `yaml:"wait_for,omitempty"`
	OnSuccess   bool              `yaml:"on_success,omitempty"`
	OnFailure   bool              `yaml:"on_failure,omitempty"`
}

type Hooks struct {
	PreStart    []Hook `yaml:"pre_start,omitempty"`
	PostStart   []Hook `yaml:"post_start,omitempty"`
	PreStop     []Hook `yaml:"pre_stop,omitempty"`
	PostStop    []Hook `yaml:"post_stop,omitempty"`
	PreBuild    []Hook `yaml:"pre_build,omitempty"`
	PostBuild   []Hook `yaml:"post_build,omitempty"`
	PreDeploy   []Hook `yaml:"pre_deploy,omitempty"`
	PostDeploy  []Hook `yaml:"post_deploy,omitempty"`
}

type Hook struct {
	Name    string            `yaml:"name"`
	Type    string            `yaml:"type"`
	Command []string          `yaml:"command,omitempty"`
	Script  string            `yaml:"script,omitempty"`
	HTTP    *HTTPHook         `yaml:"http,omitempty"`
	Exec    *ExecHook         `yaml:"exec,omitempty"`
	Timeout time.Duration     `yaml:"timeout,omitempty"`
	Retries int               `yaml:"retries,omitempty"`
}

type HTTPHook struct {
	URL     string            `yaml:"url"`
	Method  string            `yaml:"method,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Body    string            `yaml:"body,omitempty"`
}

type ExecHook struct {
	Container string   `yaml:"container"`
	Command   []string `yaml:"command"`
}

type CloudNativeConfig struct {
	Kubernetes  *KubernetesConfig  `yaml:"kubernetes,omitempty"`
	Helm        *HelmConfig        `yaml:"helm,omitempty"`
	Istio       *IstioConfig       `yaml:"istio,omitempty"`
	Prometheus  *PrometheusConfig  `yaml:"prometheus,omitempty"`
}

type KubernetesConfig struct {
	Namespace   string            `yaml:"namespace,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Resources   *Resources        `yaml:"resources,omitempty"`
}

type HelmConfig struct {
	Chart      string            `yaml:"chart"`
	Repository string            `yaml:"repository,omitempty"`
	Version    string            `yaml:"version,omitempty"`
	Values     map[string]interface{} `yaml:"values,omitempty"`
}

type IstioConfig struct {
	VirtualService  map[string]interface{} `yaml:"virtual_service,omitempty"`
	DestinationRule map[string]interface{} `yaml:"destination_rule,omitempty"`
}

type PrometheusConfig struct {
	ScrapePort     int               `yaml:"scrape_port,omitempty"`
	ScrapeInterval string            `yaml:"scrape_interval,omitempty"`
	Labels         map[string]string `yaml:"labels,omitempty"`
}

type BuildConfig struct {
	Context    string            `yaml:"context,omitempty"`
	Dockerfile string            `yaml:"dockerfile,omitempty"`
	Args       map[string]string `yaml:"args,omitempty"`
	Target     string            `yaml:"target,omitempty"`
}

type DeployConfig struct {
	Replicas  int               `yaml:"replicas,omitempty"`
	Resources *Resources        `yaml:"resources,omitempty"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}

type Resources struct {
	Limits   ResourceSpec `yaml:"limits,omitempty"`
	Requests ResourceSpec `yaml:"requests,omitempty"`
}

type ResourceSpec struct {
	CPU    string `yaml:"cpu,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

type HealthCheck struct {
	Test        []string      `yaml:"test,omitempty"`
	Interval    time.Duration `yaml:"interval,omitempty"`
	Timeout     time.Duration `yaml:"timeout,omitempty"`
	Retries     int           `yaml:"retries,omitempty"`
	StartPeriod time.Duration `yaml:"start_period,omitempty"`
}

type DependsOn struct {
	Condition string `yaml:"condition,omitempty"`
}

type Network struct {
	Driver     string            `yaml:"driver,omitempty"`
	DriverOpts map[string]string `yaml:"driver_opts,omitempty"`
	External   bool              `yaml:"external,omitempty"`
	Labels     map[string]string `yaml:"labels,omitempty"`
}

type Volume struct {
	Driver     string            `yaml:"driver,omitempty"`
	DriverOpts map[string]string `yaml:"driver_opts,omitempty"`
	External   bool              `yaml:"external,omitempty"`
	Labels     map[string]string `yaml:"labels,omitempty"`
}

type Config struct {
	File     string `yaml:"file,omitempty"`
	External bool   `yaml:"external,omitempty"`
}

type Secret struct {
	File     string `yaml:"file,omitempty"`
	External bool   `yaml:"external,omitempty"`
}