package fog

// Config defines the configuration for a project.
type Config struct {
	// Machines maps machine names to definitions
	Machines map[string]*MachineConfig
}

// MachineConfig represents the configuration for a virtual machine in a fog project.
type MachineConfig struct {
	// Image is the image name and optional tag to use
	Image string
	// Ports specifies port mappings from a host port to the VM
	Ports []string
	// CloudConfig defines cloud-config YAML for cloud-init
	CloudConfig map[string]interface{} `yaml:"cloud_config"`
}
