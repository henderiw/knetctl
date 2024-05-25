package release

type Release struct {
	// release
	Release string `yaml:"release"`
	// apps
	Apps []*Application `yaml:"apps"`
}

type Application struct {
	Name    string  `yaml:"name"`
	URL     string  `yaml:"url"`
	Image   *string `yaml:"image,omitempty"`
	Version *string `yaml:"version,omitempty"`
}
